package config

import (
	"fmt"
	"io"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type Session struct {
	In      io.Reader
	Out     io.WriteCloser
	Session *ssh.Session
}

func NewSession(username, password, host string) (*Session, error) {

	sshConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	connection, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %s", err)
	}
	session, err := connection.NewSession()
	if err != nil {
		return nil, err
	}
	sshIn, err := session.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("session stdout: %s", err)
	}
	sshOut, err := session.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("session stdin: %s", err)
	}

	if err := session.Shell(); err != nil {
		session.Close()
		return nil, fmt.Errorf("session shell: %s", err)
	}

	return &Session{
		Session: session,
		In:      sshIn,
		Out:     sshOut,
	}, nil
}

func (ses *Session) Close() {
	log.Debugf("Closing sesison")
	ses.Session.Close()
}

func (ses *Session) Expect(send, expect string, timeout int) string {
	rChan := make(chan string)

	go func() {
		buf := make([]byte, 1024)
		n, err := ses.In.Read(buf) //this reads the ssh terminal
		tmpStr := ""
		if err == nil {
			tmpStr = string(buf[:n])
		}
		for (err == nil) && (!strings.Contains(tmpStr, expect)) {
			n, err = ses.In.Read(buf)
			tmpStr += string(buf[:n])
		}
		rChan <- tmpStr
	}()

	time.Sleep(10 * time.Millisecond)

	if send != "" {
		ses.Write(send)
	}

	select {
	case ret := <-rChan:
		return ret
	case <-time.After(time.Duration(timeout) * time.Second):
		log.Warnf("timeout waiting for %s", expect)
	}
	return ""
}

func (ses *Session) Write(command string) (int, error) {
	returnCode, err := ses.Out.Write([]byte(command + "\r"))
	return returnCode, err
}

// send multiple config to a device
func SendConfig(cs []*ConfigSnippet) error {
	host := fmt.Sprintf("%s:22", cs[0].TargetNode.LongName)

	ses, err := NewSession("admin", "admin", host)
	if err != nil {
		return fmt.Errorf("cannot connect to %s: %s", host, err)
	}
	defer ses.Close()

	log.Infof("Connected to %s\n", host)
	//Read to first prompt
	ses.Expect("", "#", 1)
	// Enter config mode
	ses.Expect("/configure global", "#", 10)
	ses.Expect("discard", "#", 10)

	for _, snip := range cs {
		for _, l := range snip.Config {
			l = strings.TrimSpace(l)
			if l == "" || strings.HasPrefix(l, "#") {
				continue
			}
			ses.Expect(l, "#", 3)
			// fmt.Write("((%s))", res)
		}

		// Commit
		commit := ses.Expect("commit", "commit", 10)
		commit += ses.Expect("", "#", 10)
		log.Infof("COMMIT %s\n%s", snip, commit)
	}

	return nil
}
