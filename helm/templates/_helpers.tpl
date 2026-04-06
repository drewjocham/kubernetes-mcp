{{/*
Expand the name of the chart.
*/}}
{{- define "kube-watcher.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "kube-watcher.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every resource.
*/}}
{{- define "kube-watcher.labels" -}}
helm.sh/chart: {{ include "kube-watcher.chart" . }}
{{ include "kube-watcher.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels used by deployments, services, etc.
*/}}
{{- define "kube-watcher.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kube-watcher.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create component label for a specific component.
*/}}
{{- define "kube-watcher.componentLabel" -}}
app.kubernetes.io/component: {{ . }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kube-watcher.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{ .Values.serviceAccount.name }}
{{- else -}}
default
{{- end }}
{{- end }}

{{/*
ServiceAccount name used by watcher and mcp.
*/}}
{{- define "kube-watcher.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{ .Values.serviceAccount.name }}
{{- else -}}
default
{{- end }}
{{- end }}
