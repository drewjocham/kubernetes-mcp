package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newViewNodeStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node-status",
		Short: "Run get_node_status with explicit flags",
		RunE: func(cmd *cobra.Command, args []string) error {
			includeMetrics, _ := cmd.Flags().GetBool("include-metrics")
			taintsOnly, _ := cmd.Flags().GetBool("taints-only")
			nodeName, _ := cmd.Flags().GetString("node-name")
			output, _ := cmd.Flags().GetString("output")

			return executeViewTool(cmd, "get_node_status", map[string]any{
				"include_metrics": includeMetrics,
				"taints_only":     taintsOnly,
				"node_name":       nodeName,
			}, output)
		},
	}
	addViewRuntimeFlags(cmd)
	cmd.Flags().Bool("include-metrics", true, "include resource metrics")
	cmd.Flags().Bool("taints-only", false, "only return tainted nodes")
	cmd.Flags().String("node-name", "", "specific node to analyze")
	cmd.Flags().String("output", "yaml", "output format: yaml|json")
	return cmd
}

func newViewPodResourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pod-resources",
		Short: "Run get_pod_resources with explicit flags",
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace, _ := cmd.Flags().GetString("namespace")
			statusFilter, _ := cmd.Flags().GetString("status-filter")
			highRestartThreshold, _ := cmd.Flags().GetInt("high-restart-threshold")
			includeContainers, _ := cmd.Flags().GetBool("include-containers")
			problematicOnly, _ := cmd.Flags().GetBool("problematic-only")
			output, _ := cmd.Flags().GetString("output")

			return executeViewTool(cmd, "get_pod_resources", map[string]any{
				"namespace":              namespace,
				"status_filter":          statusFilter,
				"high_restart_threshold": highRestartThreshold,
				"include_containers":     includeContainers,
				"problematic_only":       problematicOnly,
			}, output)
		},
	}
	addViewRuntimeFlags(cmd)
	cmd.Flags().String("namespace", "", "namespace filter")
	cmd.Flags().String("status-filter", "", "filter by pod phase/status")
	cmd.Flags().Int("high-restart-threshold", 5, "restart count threshold")
	cmd.Flags().Bool("include-containers", true, "include container details")
	cmd.Flags().Bool("problematic-only", false, "only return problematic pods")
	cmd.Flags().String("output", "yaml", "output format: yaml|json")
	return cmd
}

func newViewNamespacesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespaces",
		Short: "Run list_namespaces with explicit flags",
		RunE: func(cmd *cobra.Command, args []string) error {
			includeSystem, _ := cmd.Flags().GetBool("include-system")
			includeQuotas, _ := cmd.Flags().GetBool("include-quotas")
			output, _ := cmd.Flags().GetString("output")

			return executeViewTool(cmd, "list_namespaces", map[string]any{
				"include_system": includeSystem,
				"include_quotas": includeQuotas,
			}, output)
		},
	}
	addViewRuntimeFlags(cmd)
	cmd.Flags().Bool("include-system", false, "include system namespaces")
	cmd.Flags().Bool("include-quotas", false, "include resource quota details")
	cmd.Flags().String("output", "yaml", "output format: yaml|json")
	return cmd
}

func newViewPodLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pod-logs",
		Short: "Run get_pod_logs with explicit flags",
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace, _ := cmd.Flags().GetString("namespace")
			podName, _ := cmd.Flags().GetString("pod-name")
			container, _ := cmd.Flags().GetString("container")
			tailLines, _ := cmd.Flags().GetInt("tail-lines")
			sinceSeconds, _ := cmd.Flags().GetInt("since-seconds")
			previous, _ := cmd.Flags().GetBool("previous")
			output, _ := cmd.Flags().GetString("output")

			if namespace == "" {
				return fmt.Errorf("--namespace is required")
			}
			if podName == "" {
				return fmt.Errorf("--pod-name is required")
			}

			return executeViewTool(cmd, "get_pod_logs", map[string]any{
				"namespace":     namespace,
				"pod_name":      podName,
				"container":     container,
				"tail_lines":    tailLines,
				"since_seconds": sinceSeconds,
				"previous":      previous,
			}, output)
		},
	}
	addViewRuntimeFlags(cmd)
	cmd.Flags().String("namespace", "", "namespace containing the pod")
	cmd.Flags().String("pod-name", "", "pod name")
	cmd.Flags().String("container", "", "container name (for multi-container pods)")
	cmd.Flags().Int("tail-lines", 200, "number of log lines to return")
	cmd.Flags().Int("since-seconds", 0, "only include logs newer than N seconds")
	cmd.Flags().Bool("previous", false, "return previous container logs")
	cmd.Flags().String("output", "yaml", "output format: yaml|json")
	return cmd
}

func newViewClusterAnalysisCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster-analysis",
		Short: "Run analyze_cluster with explicit flags",
		RunE: func(cmd *cobra.Command, args []string) error {
			includePods, _ := cmd.Flags().GetBool("include-pods")
			includeEvents, _ := cmd.Flags().GetBool("include-events")
			eventHoursBack, _ := cmd.Flags().GetInt("event-hours-back")
			includeServices, _ := cmd.Flags().GetBool("include-services")
			detailedAnalysis, _ := cmd.Flags().GetBool("detailed-analysis")
			output, _ := cmd.Flags().GetString("output")

			return executeViewTool(cmd, "analyze_cluster", map[string]any{
				"include_pods":      includePods,
				"include_events":    includeEvents,
				"event_hours_back":  eventHoursBack,
				"include_services":  includeServices,
				"detailed_analysis": detailedAnalysis,
			}, output)
		},
	}
	addViewRuntimeFlags(cmd)
	cmd.Flags().Bool("include-pods", true, "include pod analysis")
	cmd.Flags().Bool("include-events", true, "include event analysis")
	cmd.Flags().Int("event-hours-back", 24, "hours of events to analyze")
	cmd.Flags().Bool("include-services", false, "include service analysis")
	cmd.Flags().Bool("detailed-analysis", true, "include recommendations")
	cmd.Flags().String("output", "yaml", "output format: yaml|json")
	return cmd
}

func executeViewTool(cmd *cobra.Command, toolName string, args map[string]any, output string) error {
	runtime, _, err := initViewRuntime(cmd)
	if err != nil {
		return err
	}
	defer runtime.close()

	result, err := runtime.server.ExecuteTool(context.Background(), toolName, args)
	if err != nil {
		return fmt.Errorf("%s: %w", toolName, err)
	}
	return renderOutput(result, output)
}
