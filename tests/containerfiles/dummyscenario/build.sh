export KRKNCTL_INPUT=$(cat krknctl-input.json|tr -d "\n")
envsubst < Containerfile.template > Containerfile
podman build . -t quay.io/krkn-chaos/krknctl-test:dummy-scenario
podman push quay.io/krkn-chaos/krknctl-test:dummy-scenario