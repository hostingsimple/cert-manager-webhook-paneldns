{{- define "cert-manager-webhook-paneldns.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "cert-manager-webhook-paneldns.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "cert-manager-webhook-paneldns.labels" -}}
helm.sh/chart: {{ include "cert-manager-webhook-paneldns.name" . }}-{{ .Chart.Version }}
{{ include "cert-manager-webhook-paneldns.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "cert-manager-webhook-paneldns.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cert-manager-webhook-paneldns.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
