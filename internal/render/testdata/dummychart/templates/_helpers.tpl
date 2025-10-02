{{- define "dummy.fullname" -}}
{{- printf "%s-controller" .Release.Name -}}
{{- end -}}
