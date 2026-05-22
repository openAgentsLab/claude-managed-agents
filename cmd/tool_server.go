package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/spf13/cobra"

	"forge/internal/hands"
	"forge/internal/hands/local"
	"forge/internal/tools"
)

// newToolServerCmd runs an HTTP tool-execution server inside a Docker container.
// DockerSandbox starts the container with this command as its main process and
// calls POST /execute for each tool invocation instead of spawning a new
// "docker exec" process per call.
func newToolServerCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "tool-server",
		Short: "Run an HTTP tool-execution server (used inside Docker containers)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runToolServer(addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":7777", "listen address")
	return cmd
}

func runToolServer(addr string) error {
	ctx := context.Background()

	sb, reg, cleanup, err := local.BuildToolServerRegistry(ctx)
	if err != nil {
		return fmt.Errorf("build registry: %w", err)
	}
	defer cleanup()

	return serveTools(ctx, sb, reg, addr)
}

func serveTools(ctx context.Context, sb hands.Sandbox, reg tools.ToolRegistry, addr string) error {
	toolMap := make(map[string]einotool.InvokableTool)
	for _, t := range reg.Tools() {
		if inv, ok := t.(einotool.InvokableTool); ok {
			info, err := inv.Info(ctx)
			if err == nil && info != nil {
				toolMap[info.Name] = inv
			}
		}
	}

	token := os.Getenv("TOOL_SERVER_TOKEN")

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req toolExecRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		inv, ok := toolMap[req.Name]
		resp := toolExecResponse{}
		if !ok {
			resp.Error = fmt.Sprintf("tool %q not found", req.Name)
		} else {
			rctx := hands.WithSandbox(r.Context(), sb)
			output, execErr := inv.InvokableRun(rctx, req.Input)
			resp.Output = output
			if execErr != nil {
				resp.Error = execErr.Error()
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	fmt.Fprintf(os.Stderr, "tool-server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		return fmt.Errorf("tool-server: %w", err)
	}
	return nil
}

type toolExecRequest struct {
	Name  string `json:"name"`
	Input string `json:"input"`
}

type toolExecResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}
