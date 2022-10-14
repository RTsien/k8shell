cd $(dirname "$0") || exit
mkdir -p dist/kubeconfig
cp -r frontend dist
cd dist && go build ../cmd/shell/main.go