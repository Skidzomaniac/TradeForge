{{- define "orchestrator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "orchestrator.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "orchestrator.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "orchestrator.labels" -}}
app.kubernetes.io/name: {{ include "orchestrator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
app: orchestrator
{{- end -}}

{{- define "orchestrator.selectorLabels" -}}
app: orchestrator
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "orchestrator.serviceAccountName" -}}
{{ include "orchestrator.fullname" . }}
{{- end -}}
