{{- define "generator.labels" -}}
generator.kubernetes.io/name: {{ .Chart.Name }}
generator.kubernetes.io/instance: {{ .Release.Name }}
generator.kubernetes.io/version: {{ .Values.image.tag | quote }}
generator.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{- define "generator.selectorLabels" -}}
generator.kubernetes.io/name: {{ .Chart.Name }}
generator.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
