package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"kube-watcher/mcp/server"
)

func newViewCmd() *cobra.Command {
	viewCmd := &cobra.Command{
		Use:   "view",
		Short: "Fetch and format MCP data for human-readable terminal views",
	}

	viewCmd.AddCommand(newViewToolsCmd())
	viewCmd.AddCommand(newViewRunCmd())
	viewCmd.AddCommand(newViewHealthCmd())
	viewCmd.AddCommand(newViewStatusCmd())
	viewCmd.AddCommand(newViewInsightsCmd())
	viewCmd.AddCommand(newViewNodeStatusCmd())
	viewCmd.AddCommand(newViewPodResourcesCmd())
	viewCmd.AddCommand(newViewNamespacesCmd())
	viewCmd.AddCommand(newViewPodLogsCmd())
	viewCmd.AddCommand(newViewClusterAnalysisCmd())
	return viewCmd
}

func newViewToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List available MCP tools with formatted output",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, output, err := initViewRuntime(cmd)
			if err != nil {
				return err
			}
			defer runtime.close()
			return printMCPTools(runtime.server.ToolSummaries(), output)
		},
	}
	addViewRuntimeFlags(cmd)
	cmd.Flags().String("output", "table", "output format: table|json|yaml")
	return cmd
}

func newViewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute MCP tool and print nicely formatted output",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, output, err := initViewRuntime(cmd)
			if err != nil {
				return err
			}
			defer runtime.close()

			toolName, _ := cmd.Flags().GetString("tool")
			rawArgs, _ := cmd.Flags().GetString("args")
			if toolName == "" {
				return fmt.Errorf("--tool is required")
			}
			return runMCPTool(context.Background(), runtime.server, toolName, rawArgs, output)
		},
	}
	addViewRuntimeFlags(cmd)
	cmd.Flags().String("tool", "", "MCP tool name")
	cmd.Flags().String("args", "{}", "tool arguments JSON")
	cmd.Flags().String("output", "yaml", "output format: yaml|json")
	return cmd
}

func newViewHealthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Show MCP health output",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, output, err := initViewRuntime(cmd)
			if err != nil {
				return err
			}
			defer runtime.close()
			return renderOutput(runtime.server.HealthCheck(context.Background()), output)
		},
	}
	addViewRuntimeFlags(cmd)
	cmd.Flags().String("output", "yaml", "output format: yaml|json")
	return cmd
}

func newViewStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display node and pod status in table form",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, _, err := initViewRuntime(cmd)
			if err != nil {
				return err
			}
			defer runtime.close()

			ctx := context.Background()
			nodeResult, err := runtime.server.ExecuteTool(ctx, "get_node_status", map[string]any{"include_metrics": true})
			if err != nil {
				return fmt.Errorf("get_node_status: %w", err)
			}
			podResult, err := runtime.server.ExecuteTool(ctx, "get_pod_resources", map[string]any{"problematic_only": false, "include_containers": false})
			if err != nil {
				return fmt.Errorf("get_pod_resources: %w", err)
			}

			printNodeStatusTable(nodeResult)
			printPodSummaryTable(podResult)
			return nil
		},
	}
	addViewRuntimeFlags(cmd)
	return cmd
}

func newViewInsightsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Show trend insights from incident history",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, _, err := initViewRuntime(cmd)
			if err != nil {
				return err
			}
			defer runtime.close()

			hours, _ := cmd.Flags().GetInt("hours")
			result, err := runtime.server.ExecuteTool(context.Background(), "list_repeating_issues", map[string]any{
				"since_hours": hours,
				"limit":       50,
			})
			if err != nil {
				return fmt.Errorf("list_repeating_issues: %w", err)
			}
			return printInsightSummary(result, hours)
		},
	}
	addViewRuntimeFlags(cmd)
	cmd.Flags().Int("hours", 6, "number of hours for trend analysis")
	return cmd
}

func addViewRuntimeFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("debug", false, "enable debug logging")
	cmd.Flags().String("log-file", "", "log file path")
	cmd.Flags().String("db-path", defaultDBPath(), "history database path")
	cmd.Flags().Duration("interval", 30*time.Second, "watcher interval")
}

func initViewRuntime(cmd *cobra.Command) (*mcpRuntime, string, error) {
	v := bindCommandViper(cmd, "KW_VIEW")
	output := v.GetString("output")
	if output == "" {
		output = "table"
	}
	runtime, err := newMCPRuntime(mcpRuntimeConfig{
		debug:    v.GetBool("debug"),
		logFile:  v.GetString("log-file"),
		dbPath:   v.GetString("db-path"),
		interval: v.GetDuration("interval"),
	})
	if err != nil {
		return nil, "", err
	}
	return runtime, output, nil
}

func printMCPTools(tools []server.ToolSummary, output string) error {
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	switch output {
	case "json", "yaml":
		return renderOutput(map[string]any{"tools": tools}, output)
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "TOOL\tDESCRIPTION"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "----\t-----------"); err != nil {
			return err
		}
		for _, tool := range tools {
			if _, err := fmt.Fprintf(w, "%s\t%s\n", tool.Name, tool.Description); err != nil {
				return err
			}
		}
		return w.Flush()
	default:
		return fmt.Errorf("unsupported output format %q (expected: table|json|yaml)", output)
	}
}

func printNodeStatusTable(result map[string]any) {
	nodes := toSlice(result["nodes"])
	fmt.Println("\nNODE STATUS")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tSTATUS\tREADY\tCPU\tMEMORY\tPRESSURE_SCORE"); err != nil {
		return
	}
	if _, err := fmt.Fprintln(w, "----\t------\t-----\t---\t------\t--------------"); err != nil {
		return
	}

	for _, n := range nodes {
		node := toMap(n)
		util := toMap(node["utilization"])
		conditions := toMap(node["conditions"])

		name := fmt.Sprintf("%v", node["name"])
		status := fmt.Sprintf("%v", node["status"])
		ready := fmt.Sprintf("%v", conditions["ready"])
		cpu := fmt.Sprintf("%v", util["cpu_allocatable"])
		mem := fmt.Sprintf("%v", util["memory_allocatable"])
		score := fmt.Sprintf("%v", util["pressure_score_pct"])

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", name, status, ready, cpu, mem, score); err != nil {
			return
		}
	}
	_ = w.Flush()
}

func printPodSummaryTable(result map[string]any) {
	summary := toMap(result["summary"])
	phases := toMap(summary["phases"])

	fmt.Println("\nPOD SUMMARY")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "METRIC\tVALUE"); err != nil {
		return
	}
	if _, err := fmt.Fprintln(w, "------\t-----"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "Total Pods\t%v\n", summary["total"]); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "Running\t%v\n", phases["running"]); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "Pending\t%v\n", phases["pending"]); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "Failed\t%v\n", phases["failed"]); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "Succeeded\t%v\n", phases["succeeded"]); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "Unknown\t%v\n", phases["unknown"]); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "Problematic Count\t%d\n", len(toSlice(summary["problematic"]))); err != nil {
		return
	}
	_ = w.Flush()
}

func printInsightSummary(result map[string]any, hours int) error {
	results := toMap(result["results"])
	freq := toMap(results["frequency_trend"])

	recent := toInt(freq["recent_count"])
	previous := toInt(freq["previous_count"])
	percent := toFloat(freq["percent_change"])

	fmt.Println("INCIDENT INSIGHTS")
	fmt.Printf("window_hours: %d\n", hours)
	fmt.Printf("recent_count: %d\n", recent)
	fmt.Printf("previous_count: %d\n", previous)
	if percent > 0 {
		fmt.Printf("trend: up %.1f%% in the last %dh\n", percent, hours)
	} else {
		fmt.Printf("trend: down %.1f%% in the last %dh\n", math.Abs(percent), hours)
	}
	fmt.Printf("insight: %v\n", result["insight"])
	return nil
}

func toMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	if m, ok := v.(map[string]interface{}); ok {
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[k] = val
		}
		return out
	}
	b, _ := json.Marshal(v)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

func toSlice(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	if s, ok := v.([]interface{}); ok {
		out := make([]any, len(s))
		copy(out, s)
		return out
	}
	b, _ := json.Marshal(v)
	out := []any{}
	_ = json.Unmarshal(b, &out)
	return out
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	default:
		return 0
	}
}
