{{- define "leaderboard-api.fullname" -}}{{ printf "%s-%s" .Release.Name "leaderboard-api" | trunc 63 | trimSuffix "-" }}{{- end -}}
{{- define "leaderboard-api.labels" -}}
app: leaderboard-api
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
