PROJECT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )/..
cd $PROJECT_DIR

env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/ultrasound_linux
env CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o build/ultrasound_pi
