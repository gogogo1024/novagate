package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gogogo1024/novagate"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type configSource string

const (
	sourceDefault configSource = "default"
	sourceFile    configSource = "file"
	sourceEnv     configSource = "env"
	sourceFlag    configSource = "flag"
)

// yamlConfig is a kitex-style YAML config: load into a map, then read values via typed getters.
// It supports hierarchical keys like "server.addr".
type yamlConfig struct {
	data map[interface{}]interface{}
}

func readYAMLConfigFile(path string) (*yamlConfig, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	b, err := io.ReadAll(fd)
	if err != nil {
		return nil, err
	}

	data := make(map[interface{}]interface{})
	if err := yaml.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &yamlConfig{data: data}, nil
}

func (yc *yamlConfig) get(path string) (interface{}, bool) {
	if yc == nil {
		return nil, false
	}
	if path == "" {
		return nil, false
	}

	parts := strings.Split(path, ".")
	var cur interface{} = yc.data
	for _, p := range parts {
		switch m := cur.(type) {
		case map[interface{}]interface{}:
			v, ok := m[p]
			if !ok {
				return nil, false
			}
			cur = v
		case map[string]interface{}:
			v, ok := m[p]
			if !ok {
				return nil, false
			}
			cur = v
		default:
			return nil, false
		}
	}
	return cur, true
}

func (yc *yamlConfig) getString(path string) (string, bool, error) {
	v, ok := yc.get(path)
	if !ok {
		return "", false, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", true, fmt.Errorf("yaml %s must be string", path)
	}
	if s == "" {
		return "", true, fmt.Errorf("yaml %s is empty", path)
	}
	return s, true, nil
}

func (yc *yamlConfig) getDuration(path string) (time.Duration, bool, error) {
	s, ok, err := yc.getString(path)
	if err != nil || !ok {
		return 0, ok, err
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, true, fmt.Errorf("yaml %s invalid duration: %w", path, err)
	}
	return d, true, nil
}

type serverConfig struct {
	addr         string
	idleTimeout  time.Duration
	writeTimeout time.Duration

	addrSource         configSource
	idleTimeoutSource  configSource
	writeTimeoutSource configSource

	dotenvPath   string
	dotenvLoaded bool

	configPath   string
	configLoaded bool
}

func loadConfig() (serverConfig, error) {
	resolved, err := resolveYAML(os.Args[1:])
	if err != nil {
		return serverConfig{}, err
	}

	dotenvPath, dotenvLoaded := loadDotenv(".env")

	fileVals, err := readFileValues(resolved.yc)
	if err != nil {
		return serverConfig{}, err
	}

	envVals, err := readEnvValues()
	if err != nil {
		return serverConfig{}, err
	}

	addrDefault, idleTimeoutDefault, writeTimeoutDefault := computeDefaults(fileVals, envVals)

	fs := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ExitOnError)
	config := fs.String("config", resolved.path, "path to YAML config file")
	addr := fs.String("addr", addrDefault, "listen address")
	idleTimeout := fs.Duration("idle-timeout", idleTimeoutDefault, "connection idle timeout (0 to disable)")
	writeTimeout := fs.Duration("write-timeout", writeTimeoutDefault, "response write timeout (0 to disable)")
	_ = fs.Parse(os.Args[1:])

	flagSetFlags := visitedFlags(fs)

	finalConfigPath := *config
	if abs, err := filepath.Abs(finalConfigPath); err == nil {
		finalConfigPath = abs
	}

	return serverConfig{
		addr:         *addr,
		idleTimeout:  *idleTimeout,
		writeTimeout: *writeTimeout,
		addrSource:   pickSource(isFlagSet("addr", flagSetFlags), envVals.addrOK, fileVals.addrOK),
		idleTimeoutSource: pickSource(
			isFlagSet("idle-timeout", flagSetFlags),
			envVals.idleTimeoutOK,
			fileVals.idleTimeoutOK,
		),
		writeTimeoutSource: pickSource(
			isFlagSet("write-timeout", flagSetFlags),
			envVals.writeTimeoutOK,
			fileVals.writeTimeoutOK,
		),
		dotenvPath:   dotenvPath,
		dotenvLoaded: dotenvLoaded,
		configPath:   finalConfigPath,
		configLoaded: resolved.loaded,
	}, nil
}

func (c serverConfig) serveOptions() []novagate.ServeOption {
	return []novagate.ServeOption{
		novagate.WithIdleTimeout(c.idleTimeout),
		novagate.WithWriteTimeout(c.writeTimeout),
	}
}

func isFlagSet(name string, set map[string]bool) bool {
	return set != nil && set[name]
}

type resolvedYAML struct {
	yc     *yamlConfig
	path   string
	loaded bool
}

func resolveYAML(args []string) (resolvedYAML, error) {
	defaultConfigPath := "novagate.yaml"
	configPath, configExplicit := parseConfigPath(args, defaultConfigPath)
	if configPath == "" {
		configPath = defaultConfigPath
	}
	abs, err := filepath.Abs(configPath)
	if err == nil {
		configPath = abs
	}

	yc, err := readYAMLConfigFile(configPath)
	if err == nil {
		return resolvedYAML{yc: yc, path: configPath, loaded: true}, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		if configExplicit {
			return resolvedYAML{}, err
		}
		// Missing default config is OK.
		return resolvedYAML{yc: nil, path: configPath, loaded: false}, nil
	}
	return resolvedYAML{}, err
}

func loadDotenv(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	if err := godotenv.Load(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("load %s error: %v", path, err)
		}
		return path, false
	}
	return path, true
}

type fileValues struct {
	addr           string
	idleTimeout    time.Duration
	writeTimeout   time.Duration
	addrOK         bool
	idleTimeoutOK  bool
	writeTimeoutOK bool
}

func readFileValues(yc *yamlConfig) (fileValues, error) {
	addr, addrOK, err := yamlStringCompat(yc, "server.addr", "addr")
	if err != nil {
		return fileValues{}, err
	}
	idleTimeout, idleOK, err := yamlDurationCompat(yc, "timeouts.idle", "idle_timeout")
	if err != nil {
		return fileValues{}, err
	}
	writeTimeout, writeOK, err := yamlDurationCompat(yc, "timeouts.write", "write_timeout")
	if err != nil {
		return fileValues{}, err
	}
	return fileValues{
		addr:           addr,
		idleTimeout:    idleTimeout,
		writeTimeout:   writeTimeout,
		addrOK:         addrOK,
		idleTimeoutOK:  idleOK,
		writeTimeoutOK: writeOK,
	}, nil
}

type envValues struct {
	addr           string
	idleTimeout    time.Duration
	writeTimeout   time.Duration
	addrOK         bool
	idleTimeoutOK  bool
	writeTimeoutOK bool
}

func readEnvValues() (envValues, error) {
	addr, addrOK, err := getenvStringStrict("NOVAGATE_ADDR")
	if err != nil {
		return envValues{}, err
	}
	idleTimeout, idleOK, err := getenvDurationStrict("NOVAGATE_IDLE_TIMEOUT")
	if err != nil {
		return envValues{}, err
	}
	writeTimeout, writeOK, err := getenvDurationStrict("NOVAGATE_WRITE_TIMEOUT")
	if err != nil {
		return envValues{}, err
	}
	return envValues{
		addr:           addr,
		idleTimeout:    idleTimeout,
		writeTimeout:   writeTimeout,
		addrOK:         addrOK,
		idleTimeoutOK:  idleOK,
		writeTimeoutOK: writeOK,
	}, nil
}

func computeDefaults(fileVals fileValues, envVals envValues) (string, time.Duration, time.Duration) {
	addrDefault := ":9000"
	if fileVals.addrOK {
		addrDefault = fileVals.addr
	}
	if envVals.addrOK {
		addrDefault = envVals.addr
	}

	idleTimeoutDefault := 5 * time.Minute
	if fileVals.idleTimeoutOK {
		idleTimeoutDefault = fileVals.idleTimeout
	}
	if envVals.idleTimeoutOK {
		idleTimeoutDefault = envVals.idleTimeout
	}

	writeTimeoutDefault := 10 * time.Second
	if fileVals.writeTimeoutOK {
		writeTimeoutDefault = fileVals.writeTimeout
	}
	if envVals.writeTimeoutOK {
		writeTimeoutDefault = envVals.writeTimeout
	}

	return addrDefault, idleTimeoutDefault, writeTimeoutDefault
}

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	return set
}

func pickSource(flagSet bool, envOK bool, fileOK bool) configSource {
	if flagSet {
		return sourceFlag
	}
	if envOK {
		return sourceEnv
	}
	if fileOK {
		return sourceFile
	}
	return sourceDefault
}

func parseConfigPath(args []string, defaultValue string) (string, bool) {
	fs := flag.NewFlagSet("preconfig", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	config := fs.String("config", defaultValue, "path to YAML config file")
	_ = fs.Parse(args)
	explicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			explicit = true
		}
	})
	return *config, explicit
}

func yamlStringCompat(yc *yamlConfig, primary string, legacy string) (string, bool, error) {
	if yc == nil {
		return "", false, nil
	}
	v, ok, err := yc.getString(primary)
	if err != nil {
		return "", ok, err
	}
	if ok {
		return v, true, nil
	}
	if legacy == "" {
		return "", false, nil
	}
	return yc.getString(legacy)
}

func yamlDurationCompat(yc *yamlConfig, primary string, legacy string) (time.Duration, bool, error) {
	if yc == nil {
		return 0, false, nil
	}
	v, ok, err := yc.getDuration(primary)
	if err != nil {
		return 0, ok, err
	}
	if ok {
		return v, true, nil
	}
	if legacy == "" {
		return 0, false, nil
	}
	return yc.getDuration(legacy)
}

func getenvStringStrict(key string) (string, bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return "", false, nil
	}
	if v == "" {
		return "", true, fmt.Errorf("env %s is empty", key)
	}
	return v, true, nil
}

func getenvDurationStrict(key string) (time.Duration, bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return 0, false, nil
	}
	if v == "" {
		return 0, true, fmt.Errorf("env %s is empty", key)
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, true, fmt.Errorf("env %s invalid duration: %w", key, err)
	}
	return d, true, nil
}
