{{- define "build-worker.fullname" -}}{{ printf "%s-%s" .Release.Name "build-worker" | trunc 63 | trimSuffix "-" }}{{- end -}}
{{- define "build-worker.labels" -}}
app: build-worker
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
