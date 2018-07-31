package main

import (
	"crypto/sha256"
	"fmt"
	ldapClient "github.com/go-ldap/ldap"
	ldapServer "github.com/nmcclain/ldap"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

/*
 * constants
 */
const (
	progName    = "ldap-proxy"
	progVersion = "0.1"
	ldapTimeout = 10 * time.Second
)

/*
 * variables
 */
var (
	timezone         = os.Getenv("TZ")
	appIp            = os.Getenv("APP_IP")
	appPort          = os.Getenv("APP_PORT")
	debugEnabled     = os.Getenv("APP_DEBUG")
	targetLdapServer = os.Getenv("TARGET_LDAP_SERVER")
	targetLdapPort   = os.Getenv("TARGET_LDAP_PORT")
	Info             *log.Logger
	Error            *log.Logger
	Debug            *log.Logger
)

/*
 * structures
 */
type ldapHandler struct {
	sessions   map[string]session
	lock       sync.Mutex
	ldapServer string
	ldapPort   string
}

type session struct {
	id   string
	c    net.Conn
	ldap *ldapServer.Conn
}

/*
 * functions
 */
// function to initialize timezone
func initTimezone() {
	fmt.Printf("Initializing Timezone...\n")
	tz, err := time.LoadLocation(timezone)
	if err != nil {
		fmt.Printf("Unable to load timezones: %s\n", err)
		os.Exit(1)
	} else {
		fmt.Printf("Timezone successfully loaded %q\n", tz)
	}
}

// function to initialize the logging
func initLogger(infoHandle io.Writer, errorHandle io.Writer) {
	fmt.Printf("Initializing Logging ...\n")
	Info = log.New(infoHandle, "", 0)
	Info.SetPrefix(time.Now().Format("2006-01-02 15:04:05") + " [INFO]  ")
	Error = log.New(errorHandle, "", 0)
	Error.SetPrefix(time.Now().Format("2006-01-02 15:04:05") + " [ERROR] ")
	Debug = log.New(errorHandle, "", 0)
	Debug.SetPrefix(time.Now().Format("2006-01-02 15:04:05") + " [DEBUG] ")
	Info.Print("Logging Initialized.")
}

func debug(format string, v ...interface{}) {
	if debugEnabled == "1" {
		Debug.Printf(format, v...)
	}

}

// function to print out some variables
func printEnvVariables() {
	debug("Printing Environment Variables: ")
	for _, pair := range os.Environ() {
		debug("Env variable: %s\n", pair)
	}
}

// function to retrieve a new or existing session
func (h ldapHandler) getSession(conn net.Conn) (session, error) {
	debug("receiving session for %s", conn.LocalAddr().String())
	conn.SetDeadline(time.Now().Add(ldapTimeout))
	id := connID(conn)
	h.lock.Lock()
	s, ok := h.sessions[id] // use server connection if it exists
	h.lock.Unlock()
	if !ok { // open a new server connection if not
		debug("open new ldap connection to backend: %s:%s\n", h.ldapServer, h.ldapPort)
		l, err := ldapServer.Dial("tcp", fmt.Sprintf("%s:%s", h.ldapServer, h.ldapPort))
		if err != nil {
			return session{}, err
		}
		s = session{id: id, c: conn, ldap: l}
		h.lock.Lock()
		h.sessions[s.id] = s
		h.lock.Unlock()
	}
	return s, nil
}

// function to provide the ldap bind feature
func (h ldapHandler) Bind(bindDN, bindSimplePw string, conn net.Conn) (ldapServer.LDAPResultCode, error) {
	debug("bind request from user: %s, ip: %s", bindDN, conn.LocalAddr().String())

	defer func() {
		// recover from panic if one occured. Set err to nil otherwise.
		if err := recover(); err != nil { //catch
			Error.Printf("Exception: %v\n", err)
			os.Exit(1)
		}
	}()

	s, err := h.getSession(conn)
	if err != nil {
		return ldapServer.LDAPResultOperationsError, err
	}
	if err := s.ldap.Bind(bindDN, bindSimplePw); err != nil {
		return ldapServer.LDAPResultInvalidCredentials, err
	}
	return ldapServer.LDAPResultSuccess, nil
}

// function to provide the ldap search feature
func (h ldapHandler) Search(boundDN string, searchReq ldapServer.SearchRequest, conn net.Conn) (ldapServer.ServerSearchResult, error) {
	debug("search request from user: %s, ip: %s", boundDN, conn.LocalAddr().String())

	defer func() {
		// recover from panic if one occured. Set err to nil otherwise.
		if err := recover(); err != nil { //catch
			Error.Printf("Exception: %v\n", err)
			os.Exit(1)
		}
	}()
	s, err := h.getSession(conn)
	if err != nil {
		return ldapServer.ServerSearchResult{ResultCode: ldapServer.LDAPResultOperationsError}, nil
	}
	search := ldapServer.NewSearchRequest(
		searchReq.BaseDN,
		ldapClient.ScopeWholeSubtree,
		ldapClient.NeverDerefAliases,
		64, // size limit as number of entries
		10, // timelimit in seconds
		false,
		searchReq.Filter,
		searchReq.Attributes,
		nil)
	sr, err := s.ldap.SearchWithPaging(search, 64)
	if err != nil {
		return ldapServer.ServerSearchResult{}, err
	}
	debug("P: Search OK: %s -> num of entries = %d", search.Filter, len(sr.Entries))
	return ldapServer.ServerSearchResult{sr.Entries, []string{}, []ldapServer.Control{}, ldapServer.LDAPResultSuccess}, nil
}

// function to provide the ldap close feature
func (h ldapHandler) Close(conn net.Conn) error {
	debug("closing connection from %s", conn.LocalAddr().String())
	conn.Close() // close connection to the server when then client is closed
	h.lock.Lock()
	defer h.lock.Unlock()
	delete(h.sessions, connID(conn))
	return nil
}

// function to generate connection ids
func connID(conn net.Conn) string {
	h := sha256.New()
	connString := conn.LocalAddr().String() + conn.RemoteAddr().String()
	h.Write([]byte(connString))
	sha := fmt.Sprintf("% x", h.Sum(nil))
	debug("connection id from %s: %s", conn.LocalAddr().String(), sha)
	return string(sha)
}

// function to let the process listen on a defined ip and port
func startListener() {
	s := ldapServer.NewServer()

	handler := ldapHandler{
		sessions:   make(map[string]session),
		ldapServer: targetLdapServer,
		ldapPort:   targetLdapPort,
	}
	s.BindFunc("", handler)
	s.SearchFunc("", handler)
	// s.CloseFunc("", handler)

	// start the server
	serverPort := appIp + ":" + appPort
	Info.Printf("Listen " + progName + " on port " + serverPort)
	if err := s.ListenAndServe(serverPort); err != nil {
		Error.Print("LDAP Server Failed!")
		Error.Fatal(err.Error())
	}
}

// main function
func main() {
	// initializing
	initTimezone()
	initLogger(os.Stdout, os.Stderr)
	printEnvVariables()
	Info.Printf("Starting " + progName + " " + progVersion + " ...")

	// start application
	startListener()
}
