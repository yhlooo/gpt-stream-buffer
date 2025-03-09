FROM --platform=${TARGETPLATFORM} ubuntu:24.04
COPY gpt-stream-buffer /bin/gpt-stream-buffer
EXPOSE 80
ENTRYPOINT ["/bin/gpt-stream-buffer"]
