#!/bin/bash
# PLANTUML if there is no plantuml alias that command should be set with `java -jar $PLANTUML.LOCATION $@`
PLANTUML=${PLANTUML:-plantuml}

$PLANTUML devworkspace_che_jwt_auth.plantuml
$PLANTUML openshift-oauth.plantuml
