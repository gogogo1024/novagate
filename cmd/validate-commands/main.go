package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
)

type loc struct {
	file string
	line int
}

type issue struct {
	msg string
}

type patterns struct {
	cmdDefRe             *regexp.Regexp
	cmdDefDecimalRe      *regexp.Regexp
	registerMethodRe     *regexp.Regexp
	bridgeCallRe         *regexp.Regexp
	routerRegisterRe     *regexp.Regexp
	dispatcherRegisterRe *regexp.Regexp
}

func main() {
	os.Exit(run())
}

func run() int {
	var (
		commandsPath  = flag.String("commands", "protocol/commands.go", "path to protocol commands file")
		serverPath    = flag.String("server", "cmd/server/main.go", "path to server main.go containing setup()")
		registryPath  = flag.String("registry", "internal/service/registry.go", "path to service handler registry")
		strictAllDefs = flag.Bool("require-all", false, "if true, require every Cmd* defined in commands.go to be bridged+handled")
	)
	flag.Parse()

	scan, scanIssues := parseFixed(*commandsPath, *serverPath, *registryPath)

	issues := append([]issue{}, scanIssues...)
	issues = append(issues, validateConsistency(scan, *strictAllDefs)...)

	if len(issues) == 0 {
		fmt.Printf(
			"ok: command mappings look consistent (defs=%d mapped=%d bridged=%d handled=%d)\n",
			len(scan.cmdVals),
			len(scan.server.registered),
			len(scan.server.bridged),
			len(scan.handled),
		)
		return 0
	}

	sort.Slice(issues, func(i, j int) bool { return issues[i].msg < issues[j].msg })
	for _, it := range issues {
		fmt.Printf("- %s\n", it.msg)
	}
	return 1
}

type serverSetup struct {
	registered map[string]loc // protocol.CmdX referenced by RegisterFullMethodCommand
	bridged    map[string]loc // protocol.CmdX wired to router (bridge(...) or r.Register(...))
}

type scanResult struct {
	cmdVals     map[string]uint16
	cmdLoc      map[string]loc
	invalidDefs map[string]loc
	server      serverSetup
	handled     map[string]loc
	scanned     int
}

type cmdDefOcc struct {
	name string
	loc  loc
}

func defaultPatterns() patterns {
	return patterns{
		cmdDefRe:             regexp.MustCompile(`(?m)^\s*(Cmd[0-9A-Za-z_]+)\s+uint16\s*=\s*(0x[0-9a-fA-F]+)\s*(?://.*)?$`),
		cmdDefDecimalRe:      regexp.MustCompile(`(?m)^\s*(Cmd[0-9A-Za-z_]+)\s+uint16\s*=\s*(\d+)\s*(?://.*)?$`),
		registerMethodRe:     regexp.MustCompile(`protocol\.RegisterFullMethodCommand\(\s*"[^"]+"\s*,\s*protocol\.(Cmd[0-9A-Za-z_]+)\s*\)`),
		bridgeCallRe:         regexp.MustCompile(`\bbridge\(\s*protocol\.(Cmd[0-9A-Za-z_]+)\s*\)`),
		routerRegisterRe:     regexp.MustCompile(`\br\.Register\(\s*protocol\.(Cmd[0-9A-Za-z_]+)\s*,`),
		dispatcherRegisterRe: regexp.MustCompile(`dispatcher\.Register\(\s*protocol\.(Cmd[0-9A-Za-z_]+)\s*,`),
	}
}

func validateConsistency(scan scanResult, requireAll bool) []issue {
	var issues []issue
	issues = append(issues, validateReferencesExist(scan)...)
	issues = append(issues, validateBridgedHaveHandlers(scan)...)
	issues = append(issues, validateHandlersAreBridged(scan)...)
	issues = append(issues, validateMappingsAreBridged(scan)...)
	if requireAll {
		issues = append(issues, validateAllDefinedAreWired(scan)...)
	}
	return issues
}

func validateReferencesExist(scan scanResult) []issue {
	var issues []issue
	for name, where := range scan.server.registered {
		if _, bad := scan.invalidDefs[name]; bad {
			continue
		}
		if _, ok := scan.cmdVals[name]; !ok {
			issues = append(issues, issue{msg: fmt.Sprintf("%s:%d: server registers unknown command protocol.%s (not found as Cmd* uint16 const)", where.file, where.line, name)})
		}
	}
	for name, where := range scan.server.bridged {
		if _, bad := scan.invalidDefs[name]; bad {
			continue
		}
		if _, ok := scan.cmdVals[name]; !ok {
			issues = append(issues, issue{msg: fmt.Sprintf("%s:%d: server bridges unknown command protocol.%s (not found as Cmd* uint16 const)", where.file, where.line, name)})
		}
	}
	for name, where := range scan.handled {
		if _, bad := scan.invalidDefs[name]; bad {
			continue
		}
		if _, ok := scan.cmdVals[name]; !ok {
			issues = append(issues, issue{msg: fmt.Sprintf("%s:%d: dispatcher handles unknown command protocol.%s (not found as Cmd* uint16 const)", where.file, where.line, name)})
		}
	}
	return issues
}

func validateBridgedHaveHandlers(scan scanResult) []issue {
	var issues []issue
	for name, where := range scan.server.bridged {
		if _, ok := scan.handled[name]; !ok {
			issues = append(issues, issue{msg: fmt.Sprintf("%s:%d: command protocol.%s is bridged in server setup but has no dispatcher handler", where.file, where.line, name)})
		}
	}
	return issues
}

func validateHandlersAreBridged(scan scanResult) []issue {
	var issues []issue
	for name, where := range scan.handled {
		if _, ok := scan.server.bridged[name]; !ok {
			issues = append(issues, issue{msg: fmt.Sprintf("%s:%d: command protocol.%s has dispatcher handler but is not bridged/registered in server setup", where.file, where.line, name)})
		}
	}
	return issues
}

func validateMappingsAreBridged(scan scanResult) []issue {
	var issues []issue
	for name, where := range scan.server.registered {
		if _, ok := scan.server.bridged[name]; !ok {
			issues = append(issues, issue{msg: fmt.Sprintf("%s:%d: command protocol.%s is mapped via RegisterFullMethodCommand but is not bridged/registered to router in setup()", where.file, where.line, name)})
		}
	}
	return issues
}

func validateAllDefinedAreWired(scan scanResult) []issue {
	var issues []issue
	for name := range scan.cmdVals {
		defLoc := scan.cmdLoc[name]
		if _, ok := scan.server.bridged[name]; !ok {
			issues = append(issues, issue{msg: fmt.Sprintf("%s:%d: command protocol.%s is defined but not bridged/registered in server setup (enable -require-all only if this is intended)", defLoc.file, defLoc.line, name)})
		}
		if _, ok := scan.handled[name]; !ok {
			issues = append(issues, issue{msg: fmt.Sprintf("%s:%d: command protocol.%s is defined but has no dispatcher handler (enable -require-all only if this is intended)", defLoc.file, defLoc.line, name)})
		}
	}
	return issues
}

func parseFixed(commandsPath, serverPath, registryPath string) (scanResult, []issue) {
	pat := defaultPatterns()
	out := scanResult{
		cmdVals:     map[string]uint16{},
		cmdLoc:      map[string]loc{},
		invalidDefs: map[string]loc{},
		server:      serverSetup{registered: map[string]loc{}, bridged: map[string]loc{}},
		handled:     map[string]loc{},
	}
	byVal := map[uint16][]cmdDefOcc{}
	var issues []issue

	issues = append(issues, scanFixedFile(commandsPath, pat, &out, byVal, true)...)
	issues = append(issues, scanFixedFile(serverPath, pat, &out, byVal)...)
	issues = append(issues, scanFixedFile(registryPath, pat, &out, byVal)...)

	issues = append(issues, validateFixedExpectations(out, commandsPath, serverPath, registryPath, byVal)...)
	return out, issues
}

func scanFixedFile(path string, pat patterns, out *scanResult, byVal map[uint16][]cmdDefOcc, checkDecimal ...bool) []issue {
	b, err := os.ReadFile(path)
	if err != nil {
		return []issue{{msg: fmt.Sprintf("read %s: %v", path, err)}}
	}
	out.scanned++
	s := string(b)

	shouldCheckDecimal := false
	if len(checkDecimal) > 0 {
		shouldCheckDecimal = checkDecimal[0]
	}

	issues := scanCmdDefs(path, s, pat.cmdDefRe, out, byVal)
	if shouldCheckDecimal {
		issues = append(issues, scanDecimalCmdDefs(path, s, pat.cmdDefDecimalRe, out)...)
	}
	scanRefs(path, s, pat.registerMethodRe, out.server.registered)
	scanRefs(path, s, pat.bridgeCallRe, out.server.bridged)
	scanRefs(path, s, pat.routerRegisterRe, out.server.bridged)
	scanRefs(path, s, pat.dispatcherRegisterRe, out.handled)
	return issues
}

func scanCmdDefs(path, s string, re *regexp.Regexp, out *scanResult, byVal map[uint16][]cmdDefOcc) []issue {
	var issues []issue
	for _, mi := range re.FindAllStringSubmatchIndex(s, -1) {
		name := s[mi[2]:mi[3]]
		raw := s[mi[4]:mi[5]]
		where := loc{file: path, line: lineNumber(s, mi[0])}
		u, perr := strconv.ParseUint(raw, 0, 16)
		if perr != nil {
			issues = append(issues, issue{msg: fmt.Sprintf("%s:%d: parse %s value %q: %v", where.file, where.line, name, raw, perr)})
			continue
		}
		val := uint16(u)
		out.cmdVals[name] = val
		out.cmdLoc[name] = where
		byVal[val] = append(byVal[val], cmdDefOcc{name: name, loc: where})
	}
	return issues
}

func scanDecimalCmdDefs(path, s string, re *regexp.Regexp, out *scanResult) []issue {
	var issues []issue
	for _, mi := range re.FindAllStringSubmatchIndex(s, -1) {
		name := s[mi[2]:mi[3]]
		raw := s[mi[4]:mi[5]]
		where := loc{file: path, line: lineNumber(s, mi[0])}
		out.invalidDefs[name] = where
		issues = append(issues, issue{msg: fmt.Sprintf(
			"%s:%d: protocol.%s must be a hex literal (0x....); got decimal %q",
			where.file,
			where.line,
			name,
			raw,
		)})
	}
	return issues
}

func scanRefs(path, s string, re *regexp.Regexp, out map[string]loc) {
	for _, mi := range re.FindAllStringSubmatchIndex(s, -1) {
		name := s[mi[2]:mi[3]]
		if _, ok := out[name]; ok {
			continue
		}
		out[name] = loc{file: path, line: lineNumber(s, mi[0])}
	}
}

func validateFixedExpectations(out scanResult, commandsPath, serverPath, registryPath string, byVal map[uint16][]cmdDefOcc) []issue {
	var issues []issue
	if len(out.cmdVals) == 0 && len(out.invalidDefs) == 0 {
		issues = append(issues, issue{msg: fmt.Sprintf("no Cmd* uint16 constants found in %s", commandsPath)})
	}
	issues = append(issues, duplicateValueIssues(byVal)...)
	if len(out.server.registered) == 0 {
		issues = append(issues, issue{msg: fmt.Sprintf("no RegisterFullMethodCommand(...) found in %s", serverPath)})
	}
	if len(out.server.bridged) == 0 {
		issues = append(issues, issue{msg: fmt.Sprintf("no bridged commands found in %s (expected bridge(protocol.CmdX) or r.Register(protocol.CmdX,...))", serverPath)})
	}
	if len(out.handled) == 0 {
		issues = append(issues, issue{msg: fmt.Sprintf("no dispatcher.Register(protocol.CmdX, ...) found in %s", registryPath)})
	}
	return issues
}

func duplicateValueIssues(byVal map[uint16][]cmdDefOcc) []issue {
	var issues []issue
	for v, defs := range byVal {
		if len(defs) > 1 {
			items := make([]string, 0, len(defs))
			for _, d := range defs {
				items = append(items, fmt.Sprintf("%s(%s:%d)", d.name, d.loc.file, d.loc.line))
			}
			sort.Strings(items)
			issues = append(issues, issue{msg: fmt.Sprintf("duplicate command value 0x%04X: %v", v, items)})
		}
	}
	return issues
}

func lineNumber(s string, idx int) int {
	if idx <= 0 {
		return 1
	}
	line := 1
	for i := 0; i < idx && i < len(s); i++ {
		if s[i] == '\n' {
			line++
		}
	}
	return line
}
