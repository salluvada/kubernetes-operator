#!/bin/bash

#use relevant pod ip addresses 
while :
do
    CRUMB=$(curl http://172.17.0.5:8080/crumbIssuer/api/xml?xpath=concat\(//crumbRequestField,%22:%22,//crumb\) \
    -c cookies.txt \
    --user 'jenkins-operator:t7yWevECdSXRIia9g7D7')
    curl -s -XPOST 'http://172.17.0.5:8080/reload' --user 'jenkins-operator:t7yWevECdSXRIia9g7D7' -b cookies.txt -H $CRUMB 
    CRUMB=$(curl http://172.17.0.6:8080/crumbIssuer/api/xml?xpath=concat\(//crumbRequestField,%22:%22,//crumb\) \
    -c cookies.txt \
    --user 'jenkins-operator:t7yWevECdSXRIia9g7D7')
    curl -s -XPOST 'http://172.17.0.6:8080/reload' --user 'jenkins-operator:t7yWevECdSXRIia9g7D7' -b cookies.txt -H $CRUMB 
    echo "reloaded " $(date +"%T")
    sleep 30
done