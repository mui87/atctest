package app

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/mui87/atctest/atcoder"
)

const baseURL = "https://atcoder.jp"

type App struct {
	client  *atcoder.Client
	checker *atcoder.Checker

	contest string
	problem string
	command string

	username string
	password string

	contestURL string
	problemURL string

	outStream io.Writer
	errStream io.Writer
}

func New(args []string, outStream, errStream io.Writer) (*App, error) {
	var errBuff bytes.Buffer

	flags := flag.NewFlagSet("atctest", flag.ContinueOnError)
	flags.SetOutput(&errBuff)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(&errBuff, helpMessage)
		flags.PrintDefaults()
	}

	var (
		contest    string
		problem    string
		command    string
		username   string
		password   string
		problemURL string
		nocache    bool
	)
	flags.StringVar(&contest, "contest", "", "contest you are challenging. e.g.) ABC051")
	flags.StringVar(&problem, "problem", "", "problem you are solving. e.g.) C")
	flags.StringVar(&command, "command", "", "command to execute your program. e.g.) 'python c.py'")
	flags.StringVar(&username, "username", "", "your username of atcoder account. e.g.) 'chokudai'")
	flags.StringVar(&password, "password", "", "your password of atcoder account. e.g.) 'password'")
	flags.StringVar(&problemURL, "url", "", "url of the problem page. e.g.) 'https://abc051.contest.atcoder.jp/tasks/abc051_c'")
	flags.BoolVar(&nocache, "nocache", false, "if set, local cache of samples is not used.")
	if err := flags.Parse(args[1:]); err != nil {
		return nil, errors.New("failed to parse flags")
	}

	if problemURL == "" {
		if contest == "" {
			flags.Usage()
			return nil, fmt.Errorf("specify the contest you are challenging. e.g.) ABC051\n\n%s", errBuff.String())
		}
		if problem == "" {
			flags.Usage()
			return nil, errors.New("specify the problem you are solving. e.g.) C")
		}
		if command == "" {
			flags.Usage()
			return nil, errors.New("specify the command to execute your program. e.g.) 'python c.py'")
		}
	}

	problemURL = strings.Trim(problemURL, "'\"")

	var contestURL string
	if problemURL == "" {
		contestURL = fmt.Sprintf("%s/contests/%s", baseURL, strings.ToLower(contest))
	} else {
		contestURL = strings.TrimRight(problemURL, "/")
		i := strings.LastIndex(contestURL, "/")
		contestURL = contestURL[:i]
		i = strings.LastIndex(contestURL, "/")
		contestURL = contestURL[:i]
	}

	useCache := !nocache
	var cacheDirPath string
	home, err := homedir.Dir()
	if err != nil {
		cacheDirPath = ""
	} else {
		cacheDirPath = path.Join(home, ".atctest")
	}
	client := atcoder.NewClient(baseURL, useCache, cacheDirPath, outStream, errStream)
	if err != nil {
		return nil, err
	}

	checker := atcoder.NewChecker(outStream, errStream)

	return &App{
		client:  client,
		checker: checker,

		contest: contest,
		problem: problem,
		command: command,

		username: username,
		password: password,

		contestURL: contestURL,
		problemURL: problemURL,

		outStream: outStream,
		errStream: errStream,
	}, nil
}

func (a *App) Run() error {
	beingHeld, err := a.client.IsContestBeingHeld(a.contestURL)
	if err != nil {
		return err
	}

	if beingHeld {
		if err := a.client.LogIn(a.username, a.password); err != nil {
			return err
		} else {
			fmt.Println("login success")
		}
	}

	var problemURL string
	if a.problemURL != "" {
		problemURL = a.problemURL
	} else {
		var err error
		problemURL, err = a.client.GetProblemURL(a.contest, a.problem)
		if err != nil {
			return err
		}
	}

	samples, err := a.client.GetSamples(problemURL)
	if err != nil {
		return err
	}

	if success := a.checker.Check(a.command, samples); !success {
		return err
	}

	return nil
}

const helpMessage = `atctest is a command line tool for AtCoder.
it checks if your program correctly solve the samples provided on the problem page.

EXAMPLE: 
$ atctest -contest ABC051 -problem C -command 'python c.py'
$ atctest -url 'https://atcoder.jp/contests/abc051/tasks/abc051_c' -command 'g++ c.cpp; ./a.out'

# for contest in session, login is required to test your code
$ atctest -contest ABC127 -problem B -command 'ruby b.rb' -username mui87 -password pass1234

OPTION:`
