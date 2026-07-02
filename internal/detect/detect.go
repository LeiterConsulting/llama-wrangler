package detect

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
	"llama-wrangler/internal/ollama"
)

type Scanner struct {
	cfg config.Config
}

func New(cfg config.Config) Scanner {
	return Scanner{cfg: cfg}
}

func (s Scanner) Local(ctx context.Context) appstate.Node {
	hostname, _ := os.Hostname()
	nodeID := s.cfg.Node.NodeID
	if nodeID == "" || nodeID == "local" {
		nodeID = strings.ReplaceAll(strings.ToLower(hostname), ".", "-")
		if nodeID == "" {
			nodeID = "local"
		}
	}
	node := appstate.Node{
		NodeID:    nodeID,
		Hostname:  hostname,
		Platform:  runtime.GOOS,
		Arch:      runtime.GOARCH,
		Role:      s.cfg.Server.Mode,
		OllamaURL: s.cfg.Ollama.URL,
		Tags:      inferTags(runtime.GOOS, runtime.GOARCH),
		Status:    "healthy",
		Enabled:   true,
		Approved:  true,
		MaxJobs:   s.cfg.Limits.MaxConcurrentJobs,
		LastSeen:  time.Now().UTC(),
	}
	if s.cfg.Node.DisplayName != "" {
		node.DisplayName = s.cfg.Node.DisplayName
	} else {
		node.DisplayName = hostname
	}
	node.CPU = commandOutput("sysctl", "-n", "machdep.cpu.brand_string")
	if runtime.GOOS == "linux" {
		node.CPU = firstCPUModel()
	}
	models, available := s.detectOllama(ctx)
	node.OllamaAvailable = available
	node.Models = models
	if !available {
		node.Status = "degraded"
	}
	return node
}

func (s Scanner) detectOllama(ctx context.Context) ([]appstate.ModelState, bool) {
	client := ollama.New(s.cfg.Ollama.URL)
	tags, err := client.Tags(ctx)
	if err != nil {
		return nil, false
	}
	models := make([]appstate.ModelState, 0, len(tags.Models))
	for _, model := range tags.Models {
		models = append(models, appstate.ModelState{Name: model.Name, State: "installed"})
	}
	return models, true
}

func inferTags(goos, goarch string) []string {
	tags := []string{"local", "consensus-participant"}
	if goarch == "arm64" {
		tags = append(tags, "apple-silicon", "light-chat")
	}
	if goos == "windows" || goos == "linux" {
		tags = append(tags, "general")
	}
	return tags
}

func commandOutput(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func firstCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}
