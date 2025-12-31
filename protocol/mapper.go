package protocol

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
)

var (
	methodCommandMu sync.RWMutex
	methodCommand   = map[string]uint16{}
	commandMethod   = map[uint16]string{}
	strictMapping   bool
)

// SetStrictCommandMapping makes MapMethodToCommand return an error when
// the method is not explicitly registered.
func SetStrictCommandMapping(strict bool) {
	methodCommandMu.Lock()
	strictMapping = strict
	methodCommandMu.Unlock()
}

// RegisterMethodCommand binds a service+method to a stable protocol command ID.
// The key format is "Service.Method".
func RegisterMethodCommand(service, method string, cmd uint16) {
	RegisterFullMethodCommand(service+"."+method, cmd)
}

// RegisterFullMethodCommand binds a full method name ("Service.Method") to a stable protocol command ID.
func RegisterFullMethodCommand(fullMethod string, cmd uint16) {
	fullMethod = strings.TrimSpace(fullMethod)
	if fullMethod == "" {
		panic("RegisterFullMethodCommand: empty fullMethod")
	}
	if _, _, err := splitFullMethod(fullMethod); err != nil {
		panic("RegisterFullMethodCommand: " + err.Error())
	}

	methodCommandMu.Lock()
	if existing, ok := commandMethod[cmd]; ok && existing != fullMethod {
		panic(fmt.Sprintf("command 0x%04X already bound to %q (attempted %q)", cmd, existing, fullMethod))
	}
	methodCommand[fullMethod] = cmd
	commandMethod[cmd] = fullMethod
	methodCommandMu.Unlock()
}

func MapMethodToCommand(fullMethod string) (uint16, error) {
	fullMethod = strings.TrimSpace(fullMethod)
	service, method, err := splitFullMethod(fullMethod)
	if err != nil {
		return 0, err
	}
	normalized := service + "." + method

	methodCommandMu.RLock()
	cmd, ok := methodCommand[normalized]
	strict := strictMapping
	methodCommandMu.RUnlock()
	if ok {
		return cmd, nil
	}
	if strict {
		return 0, fmt.Errorf("unregistered command mapping for %q", normalized)
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(service))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(method))
	return uint16(h.Sum32()), nil
}

func splitFullMethod(fullMethod string) (service string, method string, err error) {
	idx := strings.LastIndexByte(fullMethod, '.')
	if idx <= 0 || idx >= len(fullMethod)-1 {
		return "", "", fmt.Errorf("invalid method format: %s", fullMethod)
	}
	service = strings.TrimSpace(fullMethod[:idx])
	method = strings.TrimSpace(fullMethod[idx+1:])
	if service == "" || method == "" {
		return "", "", fmt.Errorf("invalid method format: %s", fullMethod)
	}
	return service, method, nil
}
