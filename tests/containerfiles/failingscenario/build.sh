export KRKNCTL_INPUT=$(cat krknctl-input.json|tr -d "\n")
envsubst < Containerfile.template > Containerfile
podman build --platform linux/amd64 . -t quay.io/krkn-chaos/krknctl-test:failing-scenario
podman tag quay.io/krkn-chaos/krknctl-test:failing-scenario
podman push quay.io/krkn-chaos/krknctl-test:failing-scenario
