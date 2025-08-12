# Deployment Guide

This document outlines how to deploy and configure the StreamRecorder application in different environments.

## Environment Variables

The application supports configuration through environment variables for flexible deployment:

### Core Settings
- `WORKER_COUNT`: Number of concurrent segment downloaders per variant (default: 4)
- `REFRESH_DELAY_SECONDS`: How often to check for playlist updates in seconds (default: 3)

### NAS Transfer Settings
- `NAS_OUTPUT_PATH`: UNC path to NAS storage (default: "")
- `NAS_USERNAME`: NAS authentication username
- `NAS_PASSWORD`: NAS authentication password
- `ENABLE_NAS_TRANSFER`: Enable/disable automatic NAS transfer (default: true)

### Path Configuration
- `LOCAL_OUTPUT_DIR`: Base directory for local downloads (default: "data")
- `PROCESS_OUTPUT_DIR`: Output directory for processed videos (default: "out")

### Processing Settings
- `FFMPEG_PATH`: Path to FFmpeg executable (default: "ffmpeg")

## Docker Deployment

### Dockerfile Example

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o stream-recorder ./cmd/main

FROM alpine:latest
RUN apk --no-cache add ca-certificates ffmpeg
WORKDIR /root/
COPY --from=builder /app/stream-recorder .
CMD ["./stream-recorder"]
```

### Docker Compose Example

```yaml
version: '3.8'
services:
  stream-recorder:
    build: .
    environment:
      - NAS_OUTPUT_PATH=/mnt/nas/streams
      - NAS_USERNAME=${NAS_USERNAME}
      - NAS_PASSWORD=${NAS_PASSWORD}
      - LOCAL_OUTPUT_DIR=/app/data
      - PROCESS_OUTPUT_DIR=/app/out
      - FFMPEG_PATH=ffmpeg
    volumes:
      - ./data:/app/data
      - ./out:/app/out
      - nas_mount:/mnt/nas
    networks:
      - stream_network

volumes:
  nas_mount:
    driver: local
    driver_opts:
      type: cifs
      device: ""
      o: username=${NAS_USERNAME},password=${NAS_PASSWORD},iocharset=utf8

networks:
  stream_network:
    driver: bridge
```

## Kubernetes Deployment

### ConfigMap Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: stream-recorder-config
data:
  WORKER_COUNT: "4"
  REFRESH_DELAY_SECONDS: "3"
  ENABLE_NAS_TRANSFER: "true"
  LOCAL_OUTPUT_DIR: "/app/data"
  PROCESS_OUTPUT_DIR: "/app/out"
  FFMPEG_PATH: "ffmpeg"
```

### Secret Example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: stream-recorder-secrets
type: Opaque
data:
  NAS_USERNAME: <base64-encoded-username>
  NAS_PASSWORD: <base64-encoded-password>
```

### Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: stream-recorder
spec:
  replicas: 1
  selector:
    matchLabels:
      app: stream-recorder
  template:
    metadata:
      labels:
        app: stream-recorder
    spec:
      containers:
      - name: stream-recorder
        image: stream-recorder:latest
        envFrom:
        - configMapRef:
            name: stream-recorder-config
        - secretRef:
            name: stream-recorder-secrets
        volumeMounts:
        - name: data-storage
          mountPath: /app/data
        - name: output-storage
          mountPath: /app/out
      volumes:
      - name: data-storage
        persistentVolumeClaim:
          claimName: stream-data-pvc
      - name: output-storage
        persistentVolumeClaim:
          claimName: stream-output-pvc
```

## Production Considerations

### Security
- Never commit credentials to version control
- Use environment variables or secret management systems for sensitive data
- Consider using service accounts or IAM roles for cloud deployments
- Rotate credentials regularly

### Monitoring
- Implement health checks for the application
- Monitor disk space for download directories
- Set up alerts for failed transfers or processing
- Log to centralized logging systems

### Scaling
- Use horizontal scaling for multiple concurrent streams
- Consider using message queues for segment processing
- Implement distributed storage for high availability
- Use load balancers for multiple instances

### Backup and Recovery
- Regular backups of configuration and state files
- Test recovery procedures
- Document rollback processes
- Maintain disaster recovery plans

## Configuration Validation

The application validates configuration at startup and will fail fast if:
- Required directories cannot be created
- NAS paths are invalid when transfer is enabled
- FFmpeg is not found when processing is enabled
- Critical environment variables are malformed

## Troubleshooting

### Common Issues
1. **Path Permission Errors**: Ensure the application has write access to configured directories
2. **NAS Connection Failures**: Verify network connectivity and credentials
3. **FFmpeg Not Found**: Install FFmpeg or set correct FFMPEG_PATH
4. **Environment Variable Format**: Check for typos and correct boolean values

### Debug Mode
Run with `-debug=true` to enable debug logging and download only 1080p variants for testing.