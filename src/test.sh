#!/bin/sh

set -e
set -u
set -x 

nohup sh -c /ldap_server &
sleep 4
TARGET_LDAP_SERVER=localhost TARGET_LDAP_PORT=3389 /ldap_search

nohup sh -c APP_DEBUG=1 TARGET_LDAP_SERVER=localhost TARGET_LDAP_PORT=3389 APP_IP=0.0.0.0 APP_PORT=3390 /ldap_proxy > &2 
sleep 14
TARGET_LDAP_SERVER=localhost TARGET_LDAP_PORT=3390 /ldap_search


