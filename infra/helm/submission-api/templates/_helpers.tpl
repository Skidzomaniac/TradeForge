{{- define "submission-api.fullname" -}}{{ printf "%s-%s" .Release.Name "submission-api" | trunc 63 | trimSuffix "-" }}{{- end -}}
{{- define "submission-api.labels" -}}
app: submission-api
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
