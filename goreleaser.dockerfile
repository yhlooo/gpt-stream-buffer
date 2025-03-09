FROM --platform=${TARGETPLATFORM} busybox:latest
COPY gpt-stream-buffer /bin/gpt-stream-buffer
EXPOSE 80
ENTRYPOINT ["/bin/gpt-stream-buffer"]
