{{- define "frontend.fullname" -}}{{ printf "%s-%s" .Release.Name "frontend" | trunc 63 | trimSuffix "-" }}{{- end -}}
{{- define "frontend.labels" -}}
app: frontend
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
