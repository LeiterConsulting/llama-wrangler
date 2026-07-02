package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"llama-wrangler/internal/config"
	"llama-wrangler/internal/httpapi"
	"llama-wrangler/internal/servicewrap"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		runDefault()
		return
	}

	switch os.Args[1] {
	case "marshal", "subscriber", "standalone":
		runMode(os.Args[1], os.Args[2:])
	case "start":
		runDefault()
	case "open":
		fmt.Println("Open the setup UI: http://localhost:11435/ui")
	case "install":
		fmt.Println("Llama Wrangler install helper")
		fmt.Println("For this MVP, run `llama-wrangler start` and complete setup in the UI.")
	case "service-dry-run":
		runServiceDryRun(os.Args[2:])
	case "uninstall":
		fmt.Println("No service was removed by this MVP helper. Delete the app data directory from the Settings page or manually if needed.")
	case "version", "--version", "-v":
		fmt.Println(version)
	default:
		usage()
		os.Exit(2)
	}
}

func runServiceDryRun(args []string) {
	fs := flag.NewFlagSet("service-dry-run", flag.ExitOnError)
	var env serviceEnvFlags
	target := fs.String("target", servicewrap.TargetLaunchd, "Service wrapper target: launchd")
	binaryPath := fs.String("binary", "", "Path to llama-wrangler binary")
	mode := fs.String("mode", "start", "Service command mode: start, marshal, subscriber, or standalone")
	configPath := fs.String("config", "", "Optional config path for marshal/subscriber/standalone modes")
	keychain := fs.Bool("keychain", false, "Include OS keychain opt-in environment")
	label := fs.String("label", "com.llama-wrangler.marshal", "Service label")
	workingDir := fs.String("working-dir", "", "Optional working directory")
	logDir := fs.String("log-dir", "~/Library/Logs/Llama Wrangler", "Launchd log directory")
	keepAlive := fs.Bool("keep-alive", false, "Set launchd KeepAlive")
	runAtLoad := fs.Bool("run-at-load", true, "Set launchd RunAtLoad")
	fs.Var(&env, "env", "Additional non-secret environment variable in KEY=VALUE form")
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	if *binaryPath == "" {
		if current, err := os.Executable(); err == nil {
			*binaryPath = current
		}
	}
	plan, err := servicewrap.NewDryRunPlan(servicewrap.Options{
		Target:         *target,
		BinaryPath:     *binaryPath,
		ConfigPath:     *configPath,
		Mode:           *mode,
		EnableKeychain: *keychain,
		Environment:    env.Values(),
		Label:          *label,
		WorkingDir:     *workingDir,
		LogDir:         *logDir,
		KeepAlive:      *keepAlive,
		RunAtLoad:      *runAtLoad,
	})
	if err != nil {
		log.Fatal(err)
	}
	raw, err := servicewrap.MarshalJSON(plan)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(raw))
}

type serviceEnvFlags []string

func (f *serviceEnvFlags) String() string {
	return strings.Join(*f, ",")
}

func (f *serviceEnvFlags) Set(value string) error {
	if !strings.Contains(value, "=") {
		return fmt.Errorf("environment value must use KEY=VALUE")
	}
	*f = append(*f, value)
	return nil
}

func (f *serviceEnvFlags) Values() map[string]string {
	values := map[string]string{}
	for _, item := range *f {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		values[key] = value
	}
	return values
}

func runDefault() {
	cfg := config.Default("marshal")
	runServer(cfg)
}

func runMode(mode string, args []string) {
	fs := flag.NewFlagSet(mode, flag.ExitOnError)
	configPath := fs.String("config", "", "Path to YAML config")
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}

	cfg := config.Default(mode)
	if *configPath != "" {
		loaded, err := config.Load(*configPath)
		if err != nil {
			log.Fatalf("load config: %v", err)
		}
		cfg = loaded
	}
	cfg.Server.Mode = mode
	if cfg.Version == "" {
		cfg.Version = version
	}
	runServer(cfg)
}

func runServer(cfg config.Config) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := httpapi.NewServer(cfg)
	if err != nil {
		log.Fatalf("initialize server: %v", err)
	}

	fmt.Println("Llama Wrangler is running.")
	fmt.Printf("Open the setup UI: http://localhost%s/ui\n", displayPort(cfg.Server.Listen))
	if token := server.RecoveryAdminToken(); token != "" {
		fmt.Printf("Local admin recovery token for this startup: %s\n", token)
	}

	if err := server.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

func displayPort(listen string) string {
	for i := len(listen) - 1; i >= 0; i-- {
		if listen[i] == ':' {
			return listen[i:]
		}
	}
	return ":11435"
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  llama-wrangler start")
	fmt.Println("  llama-wrangler marshal --config ./marshal.yaml")
	fmt.Println("  llama-wrangler subscriber --config ./subscriber.yaml")
	fmt.Println("  llama-wrangler standalone --config ./standalone.yaml")
	fmt.Println("  llama-wrangler service-dry-run --target launchd --keychain")
	fmt.Println("  llama-wrangler install|open|uninstall|version")
}
