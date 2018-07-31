// +build ignore

// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/nmcclain/ldap"
)

var (
	targetLdapServer          = os.Getenv("TARGET_LDAP_SERVER")
	targetLdapPort            = os.Getenv("TARGET_LDAP_PORT")
	baseDN           string   = "dc=*,dc=*"
	filter           string   = "(&(objectClass=user)(sAMAccountName=*)(memberOf=CN=*,OU=*,DC=*,DC=*))"
	Attributes       []string = []string{"memberof"}
	user             string   = "test1234"
	passwd           string   = "test1234"
)

func main() {
	l, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", targetLdapServer, targetLdapPort))
	if err != nil {
		log.Fatalf("ERROR: %s\n", err.Error())
	}
	defer l.Close()
	// l.Debug = true

	err = l.Bind(user, passwd)
	if err != nil {
		log.Printf("ERROR: Cannot bind: %s\n", err.Error())
		return
	}
	search := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		Attributes,
		nil)

	sr, err := l.Search(search)
	if err != nil {
		log.Fatalf("ERROR: %s\n", err.Error())
		return
	}

	log.Printf("Search: %s -> num of entries = %d\n", search.Filter, len(sr.Entries))
	sr.PrettyPrint(0)
}
