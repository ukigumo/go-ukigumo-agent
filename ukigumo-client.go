package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"time"

	flags "github.com/jessevdk/go-flags"
)

const (
	STATUS_SUCCESS = "1"
	STATUS_FAIL    = "2"
	STATUS_NA      = "3"
	STATUS_SKIP    = "4"
	STATUS_PENDING = "5"
	STATUS_TIMEOUT = "6"
)

func RunCommand(buf *bytes.Buffer, cmd string, args ...string) error {
	buf.Write([]byte("\n++ " + time.Now().String() + " ++\n"))

	c := exec.Command(cmd, args...)
	out, err := c.CombinedOutput()
	buf.Write(out)
	return err
}

/**
* Perl project using Module::Build
 */
func RunBuildPL(buf *bytes.Buffer) (string, []byte, error) {
	err := RunCommand(buf, "perl", "Build.PL")
	if err != nil {
		return STATUS_NA, buf.Bytes(), err
	}

	err = RunCommand(buf, "./Build")
	if err != nil {
		return STATUS_NA, buf.Bytes(), err
	}

	err = RunCommand(buf, "./Build", "test")
	if err != nil {
		return STATUS_FAIL, buf.Bytes(), err
	}

	return STATUS_SUCCESS, buf.Bytes(), nil
}

/**
* Perl project using ExtUtils::MakeMaker
 */
func RunMakefilePL(buf *bytes.Buffer) (string, []byte, error) {
	err := RunCommand(buf, "perl", "Makefile.PL")
	if err != nil {
		return STATUS_NA, buf.Bytes(), err
	}

	err = RunCommand(buf, "make")
	if err != nil {
		return STATUS_NA, buf.Bytes(), err
	}

	err = RunCommand(buf, "make", "test")
	if err != nil {
		return STATUS_FAIL, buf.Bytes(), err
	}

	return STATUS_SUCCESS, buf.Bytes(), nil
}

/**
 * Run test suites.
 */
func RunTests() (string, []byte, error) {
	buf := bytes.NewBuffer(nil)

	if _, err := os.Stat(".ukigumo.yml"); err == nil {
		return STATUS_NA, buf.Bytes(), fmt.Errorf(".ukigumo.yml is not supported yet")
	} else if _, err := os.Stat("Build.PL"); err == nil {
		return RunBuildPL(buf)
	} else if _, err := os.Stat("Makefile.PL"); err == nil {
		return RunMakefilePL(buf)
	} else {
		return STATUS_NA, buf.Bytes(), fmt.Errorf("Unknown project type. There is no .ukigumo.yml, Makefile.PL or Build.PL.")
	}
}

var version = "v0.0.1"

type cmdOptions struct {
	Help     bool   `short:"h" long:"help" description:"show this help message and exit"`
	EndPoint string `long:"endpoint" description:"End Point"`
	Project  string `long:"project" description:"project name"`
	Version  bool   `long:"version" description:"print the version and exit"`
}

func main() {
	opts := &cmdOptions{}
	parser := flags.NewParser(opts, flags.PrintErrors)
	parser.Name = "ukigumo-client"
	parser.Usage = "[OPTIONS]"

	_, err := parser.Parse()

	if err != nil {
		parser.WriteHelp(os.Stdout)
		os.Exit(1)
	}

	if opts.Help {
		parser.WriteHelp(os.Stdout)
		os.Exit(1)
	}

	if opts.Version {
		fmt.Fprintf(os.Stderr, "ukigumo-client: %s\n", version)
		return
	}

	if len(opts.EndPoint) == 0 {
		log.Fatal("You must set endpoint url by arguments.")
	}
	endpoint := opts.EndPoint

	project := opts.Project
	if len(project) == 0 {
		pwd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}

		project = path.Base(pwd)
	}

	log.Printf("Endpoint: %s", endpoint)
	status, out, err := RunTests()
	log.Printf("status: %s", status)
	if err == nil {
		// succeeded.
		log.Printf("out: %s", out)

		resp, err := http.PostForm(endpoint,
			url.Values{
				"status":   {status},
				"project":  {project},
				"branch":   {"master"},
				"vc_log":   {""},
				"body":     {string(out[:])},
				"revision": {"1"},
				"repo":     {"http://example.com"},
			})
		if err != nil {
			log.Fatal(err)
		}
		log.Print(resp)
	} else {
		// failed.
		log.Print(err)
		log.Print(string(out[:]))

		resp, err := http.PostForm(endpoint,
			url.Values{
				"status":   {status},
				"project":  {project},
				"branch":   {"master"},
				"vc_log":   {""},
				"body":     {string(out[:])},
				"revision": {"1"},
				"repo":     {"http://example.com"},
			})
		if err != nil {
			log.Fatal(err)
		}
		log.Print(resp)
	}
}
