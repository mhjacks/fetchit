package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-co-op/gocron"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/redhat-et/harpoon/pkg/engine"
)

type Repo struct {
	Url          string
	Branch       string
	Method       string
	Subdirectory string
	Schedule     string
}

func main() {
	repoJson, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}
	var repo Repo
	json.Unmarshal([]byte(repoJson), &repo)

	s := gocron.NewScheduler(time.UTC)
	s.Cron(repo.Schedule).Do(process)
	s.StartAsync()
	select {}
}

func process() {
	repoJson, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}
	var repo Repo
	json.Unmarshal([]byte(repoJson), &repo)

	directory := filepath.Base(repo.Url)
	r, err := git.PlainClone(directory, false, &git.CloneOptions{
		URL:           repo.Url,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", repo.Branch)),
		SingleBranch:  true,
	})
	if err != nil && err != git.ErrRepositoryAlreadyExists {
		fmt.Printf("Error while cloning the repository: %s\n", err)
	}
	ref, err := r.Head()
	if err != nil {
		fmt.Printf("Error when retrieving head: %s\n", err)
	}

	// ... retrieving the commit object
	commit, err := r.CommitObject(ref.Hash())
	if err != nil {
		fmt.Printf("Error when retrieving commit: %s\n", err)
	}

	// ... retrieve the tree from the commit
	tree, err := commit.Tree()
	if err != nil {
		log.Fatal(err)
	}

	// ... get the files iterator and print the file
	tree.Files().ForEach(func(f *object.File) error {
		if strings.Contains(f.Name, repo.Subdirectory) {
			path := directory + "/" + f.Name
			engine.EngineMethod(path, repo.Method)
		}
		return nil
	})

	// Pull the latest changes from the remote
	fmt.Printf("Pulling latest repository changes from %s branch %s\n", repo.Url, repo.Branch)

	// Open the local repository
	r, err = git.PlainOpen(directory)
	if err != nil {
		log.Fatal(err)
	}

	w, err := r.Worktree()
	if err != nil {
		log.Fatal(err)
	}

	// ... retrieving the commit object
	prevCommit, err := r.CommitObject(ref.Hash())
	if err != nil {
		fmt.Printf("Error when retrieving commit: %s\n", err)
	}

	// ... retrieve the tree from the commit
	prevTree, err := prevCommit.Tree()
	if err != nil {
		fmt.Printf("Error while generating tree: %s\n", err)
	}

	// Pull the latest changes from the origin remote and merge into the current branch
	err = w.Pull(&git.PullOptions{
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", repo.Branch)),
		SingleBranch:  true,
	})
	if err != nil {
		fmt.Printf("Nothing to pull.....Requeuing \n")
	} else {
		// Print the latest commit that was just pulled
		ref, err := r.Head()
		if err != nil {
			fmt.Printf("An error has occured %s\n", err)
		}
		commit, err := r.CommitObject(ref.Hash())
		if err != nil {
			fmt.Printf("Error when retrieving commit: %s\n", err)
		}

		// ... retrieve the tree from the commit
		tree, err := commit.Tree()
		if err != nil {
			fmt.Printf("Error while generating tree: %s\n", err)
		}

		changes, err := tree.Diff(prevTree)
		if err != nil {
			log.Fatal(err)
		}
		for _, change := range changes {
			if strings.Contains(change.To.Name, repo.Subdirectory) {
				path := directory + "/" + change.To.Name
				engine.EngineMethod(path, repo.Method)
			}
		}
	}
}