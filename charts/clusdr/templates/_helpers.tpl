{{/*
Expand the name of the chart.
*/}}
{{- define "clusdr.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "clusdr.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "clusdr.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "clusdr.labels" -}}
helm.sh/chart: {{ include "clusdr.chart" . }}
{{ include "clusdr.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: clusdr
{{- end }}

{{- define "clusdr.selectorLabels" -}}
app.kubernetes.io/name: {{ include "clusdr.name" . }}
{{- end }}

{{- define "clusdr.image" -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) }}
{{- end }}

{{- define "clusdr.voterCount" -}}
{{- $n := int .Values.voterCount }}
{{- if or (lt $n 1) (eq (mod $n 2) 0) }}
{{- fail "voterCount must be an odd integer >= 1 (Helm does not join; NOTES uses this count)" }}
{{- end }}
{{- $n }}
{{- end }}

{{- define "clusdr.prepareImage" -}}
{{- printf "%s:%s" .Values.prepare.image.repository .Values.prepare.image.tag }}
{{- end }}

{{- define "clusdr.prepareInit" -}}
initContainers:
  - name: prepare
    image: {{ include "clusdr.prepareImage" . }}
    imagePullPolicy: {{ .Values.prepare.image.pullPolicy }}
    command: ["sh", "-c", "mkdir -p \"$CLUSDR_DATA_DIR\" && chown 65532:65532 \"$CLUSDR_DATA_DIR\""]
    env:
      - name: CLUSDR_DATA_DIR
        value: {{ .Values.dataDir | quote }}
    securityContext:
      runAsUser: 0
      runAsGroup: 0
    volumeMounts:
      - name: data
        mountPath: {{ .Values.dataDir }}
{{- end }}

{{- define "clusdr.probes" -}}
{{- if eq .Values.probes.type "tcp" }}
livenessProbe:
  tcpSocket:
    port: 7947
  initialDelaySeconds: 3
  periodSeconds: 10
  timeoutSeconds: 5
readinessProbe:
  tcpSocket:
    port: 7947
  initialDelaySeconds: 2
  periodSeconds: 5
  timeoutSeconds: 5
{{- else }}
livenessProbe:
  exec:
    command: ["/clusdr", "health"]
  initialDelaySeconds: 3
  periodSeconds: 10
  timeoutSeconds: 5
readinessProbe:
  exec:
    command: ["/clusdr", "health"]
  initialDelaySeconds: 2
  periodSeconds: 5
  timeoutSeconds: 5
{{- end }}
{{- end }}
