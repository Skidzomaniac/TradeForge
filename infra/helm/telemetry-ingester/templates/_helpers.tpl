{{- define "telemetry-ingester.fullname" -}}{{ printf "%s-%s" .Release.Name "telemetry-ingester" | trunc 63 | trimSuffix "-" }}{{- end -}}
{{- define "telemetry-ingester.labels" -}}
app: telemetry-ingester
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
