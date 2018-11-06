/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package commands

import (
	"bufio"
	"bytes"
	gocontext "context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/containerd/console"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// PushTracker returns a new InMemoryTracker which tracks the ref status
var PushTracker = docker.NewInMemoryTracker()

func passwordPrompt() (string, error) {
	c := console.Current()
	defer c.Reset()

	if err := c.DisableEcho(); err != nil {
		return "", errors.Wrap(err, "failed to disable echo")
	}

	line, _, err := bufio.NewReader(c).ReadLine()
	if err != nil {
		return "", errors.Wrap(err, "failed to read line")
	}
	return string(line), nil
}

type authConfig struct {
	Auths map[string]authRecord `json:"auths"`
}

type authRecord struct {
	Auth string `json:"auth"`
}

// GetResolver prepares the resolver from the environment and options
func GetResolver(ctx gocontext.Context, clicontext *cli.Context) (remotes.Resolver, error) {
	username := clicontext.String("user")
	var secret string
	if i := strings.IndexByte(username, ':'); !strings.HasPrefix(username, "file:") && i > 0 {
		secret = username[i+1:]
		username = username[0:i]
	}
	options := docker.ResolverOptions{
		PlainHTTP: clicontext.Bool("plain-http"),
		Tracker:   PushTracker,
	}
	var cfg authConfig
	if strings.HasPrefix(username, "file:") {
		username = strings.TrimPrefix(username, "file:")
		dt, err := ioutil.ReadFile(username)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(dt, &cfg); err != nil {
			return nil, err
		}
	} else if username != "" {
		if secret == "" {
			fmt.Printf("Password: ")

			var err error
			secret, err = passwordPrompt()
			if err != nil {
				return nil, err
			}

			fmt.Print("\n")
		}
	} else if rt := clicontext.String("refresh"); rt != "" {
		secret = rt
	}

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: clicontext.Bool("skip-verify"),
		},
		ExpectContinueTimeout: 5 * time.Second,
	}

	options.Client = &http.Client{
		Transport: tr,
	}

	credentials := func(host string) (string, string, error) {
		if host == "registry-1.docker.io" {
			host = "https://index.docker.io/v1/"
		}
		ac, ok := cfg.Auths[host]
		if ok {
			dt, err := base64.StdEncoding.DecodeString(ac.Auth)
			if err != nil {
				return "", "", err
			}
			parts := bytes.SplitN(dt, []byte(":"), 2)
			if len(parts) != 2 {
				return "", "", errors.Errorf("invalid auth config")
			}
			username = string(parts[0])
			secret = string(parts[1])
		}
		// Only one host
		return username, secret, nil
	}
	options.Authorizer = docker.NewAuthorizer(options.Client, credentials)

	return docker.NewResolver(options), nil
}
