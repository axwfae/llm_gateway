podman image rm -fa
podman build -t llm:v1 .
podman tag llm:v1 wuyong1977/llm_gateway:latest
podman push wuyong1977/llm_gateway:latest
