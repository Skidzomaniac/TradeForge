{{- define "bot-fleet.fullname" -}}{{ printf "%s-%s" .Release.Name "bot-fleet" | trunc 63 | trimSuffix "-" }}{{- end -}}
{{- define "bot-fleet.labels" -}}
app: bot-fleet
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
