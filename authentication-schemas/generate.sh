#!/bin/bash
# PLANTUML if there is no plantuml alias that command should be set with `java -jar $PLANTUML.LOCATION $@`
PLANTUML=${PLANTUML:-plantuml}

$PLANTUML jwtproxy-current.plantuml
$PLANTUML jwtproxy+auth_bridge-next.plantuml
$PLANTUML openid-next.plantuml
