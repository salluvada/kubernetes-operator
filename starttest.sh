#! /bin/bash

docker build -t jenkins-operator --build-arg GO_VERSION=1.13.9 --build-arg OPERATOR_SDK_VERSION=0.15.1 -f ./build/Dockerfile .
docker save "jenkins-operator" | pv | (eval $(minikube docker-env) && docker load)
helm install jenkins-operator --set image.repository=jenkins-operator --set image.tag=latest ./chart/jenkins-operator/
