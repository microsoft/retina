{{- define "retina-rust.namespace" -}}
{{ .Values.namespace }}
{{- end -}}

{{- define "retina-rust.labels" -}}
app.kubernetes.io/managed-by: Helm
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "retina-rust.agent.selectorLabels" -}}
app: retina-agent
{{- end -}}

{{- define "retina-rust.operator.selectorLabels" -}}
app: retina-operator
{{- end -}}

{{- define "retina-rust.operator.addr" -}}
http://retina-operator.{{ include "retina-rust.namespace" . }}.svc.cluster.local:{{ .Values.operator.grpcPort }}
{{- end -}}
