package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"path"

	"fmt"
	"html/template"
	"os"
	"os/exec"
	"os/user"
	"time"

	flags "github.com/jessevdk/go-flags"
)

// Endpoint URL
var endpoint string
var workDir string

const (
	StatusSuccess = "1"
	StatusFail    = "2"
	StatusNA      = "3"
	StatusSkip    = "4"
	StatusPending = "5"
	StatusTimeout = "6"
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
		return StatusNA, buf.Bytes(), err
	}

	err = RunCommand(buf, "./Build")
	if err != nil {
		return StatusNA, buf.Bytes(), err
	}

	err = RunCommand(buf, "./Build", "test")
	if err != nil {
		return StatusFail, buf.Bytes(), err
	}

	return StatusSuccess, buf.Bytes(), nil
}

/**
* Perl project using ExtUtils::MakeMaker
 */
func RunMakefilePL(buf *bytes.Buffer) (string, []byte, error) {
	err := RunCommand(buf, "perl", "Makefile.PL")
	if err != nil {
		return StatusNA, buf.Bytes(), err
	}

	err = RunCommand(buf, "make")
	if err != nil {
		return StatusNA, buf.Bytes(), err
	}

	err = RunCommand(buf, "make", "test")
	if err != nil {
		return StatusFail, buf.Bytes(), err
	}

	return StatusSuccess, buf.Bytes(), nil
}

/**
 * Run test suites.
 */
func RunTests() (string, []byte, error) {
	buf := bytes.NewBuffer(nil)

	if _, err := os.Stat(".ukigumo.yml"); err == nil {
		return StatusNA, buf.Bytes(), fmt.Errorf(".ukigumo.yml is not supported yet")
	} else if _, err := os.Stat("Build.PL"); err == nil {
		return RunBuildPL(buf)
	} else if _, err := os.Stat("Makefile.PL"); err == nil {
		return RunMakefilePL(buf)
	} else {
		return StatusNA, buf.Bytes(), fmt.Errorf("Unknown project type. There is no .ukigumo.yml, Makefile.PL or Build.PL.")
	}
}

var version = "v0.0.1"

type Response map[string]interface{}

func (r Response) String() (s string) {
	b, err := json.Marshal(r)
	if err != nil {
		s = ""
		return
	}
	s = string(b)
	return
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, _ := template.ParseFiles("index.html")
	t.Execute(w, nil)
}

func docsApiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, _ := template.ParseFiles("docs-api.html")
	t.Execute(w, nil)
}

func runWorker(repository string, branch string) {
	buf := bytes.NewBuffer(nil)

	// TODO we should care the directory traversal
	tmpdir := path.Join(workDir, url.QueryEscape(repository), url.QueryEscape(branch))
	os.RemoveAll(tmpdir)

	log.Printf("Testing %s@%s to %s", repository, branch, tmpdir)

	// git clone
	os.MkdirAll(tmpdir, 0777)
	err := RunCommand(buf, "git", "clone", "--branch", branch, repository, tmpdir)
	if err != nil {
		log.Printf("git clone failed: %s", buf.Bytes()[:])
		return
	}

	log.Printf("run tests")

	log.Printf("... Finished!")
}

func enqueueHandler(w http.ResponseWriter, r *http.Request) {
	repository := r.FormValue("repository")
	branch := r.FormValue("branch")

	if len(repository) == 0 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, Response{"success": false, "message": "Missing mandatory parameter: repository"})
		return
	}

	if len(branch) == 0 {
		branch = "master"
	}

	go runWorker(repository, branch)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprint(w, Response{"success": true, "message": "enqueued."})
}

func startServer() {
	// TODO /api/github_hook
	http.HandleFunc("/", handler)
	http.HandleFunc("/docs/api", docsApiHandler)
	http.HandleFunc("/api/v0/enqueue", enqueueHandler)
	http.ListenAndServe(":8080", nil)
}

type cmdOptions struct {
	Help     bool   `short:"h" long:"help" description:"show this help message and exit"`
	EndPoint string `long:"endpoint" description:"End Point"`
	WorkDir  string `long:"workdir" description:"Working directory"`
	Project  string `long:"project" description:"project name"`
	Version  bool   `long:"version" description:"print the version and exit"`
}

// curl -X POST http://localhost:8080/api/v0/enqueue\?repository\=git://github.com/tokuhirom/Acme-Failing.git
func main() {
	opts := &cmdOptions{}
	parser := flags.NewParser(opts, flags.PrintErrors)
	parser.Name = "ukigumo-agent"
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
		fmt.Fprintf(os.Stderr, "ukigumo-agent: %s\n", version)
		return
	}
	if len(opts.EndPoint) == 0 {
		log.Fatal("You must set endpoint url by arguments.")
	}
	endpoint = opts.EndPoint

	if len(opts.WorkDir) == 0 {
		usr, err := user.Current()
		if err != nil {
			log.Fatal(err)
		}
		workDir = path.Join(usr.HomeDir, ".go-ukigumo-agent")
	} else {
		workDir = opts.WorkDir
	}
	err = os.MkdirAll(workDir, 0777)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("working directory is: " + workDir)

	startServer()

	/*

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
	*/
}
