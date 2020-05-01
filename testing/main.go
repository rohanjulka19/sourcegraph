package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

type Repo struct {
	owner string
	name  string
	revs  []string
}

var repos = []Repo{
	{
		owner: "uber-go",
		name:  "zap",
		revs: []string{
			// no go.mod, only glide
			// "35aad584952c3e7020db7b839f6b102de6271f89", // v1.7.1
			// "ff33455a0e382e8a81d14dd7c922020b6b5e7982", // v1.9.1
			// "27376062155ad36be76b0f12cf1572a221d3a48c", // v1.10.0
			"a6015e13fab9b744d96085308ce4e8f11bad1996", // v1.12.0
			"2aa9fa25da83bdfff756c36a91442edc9a84576c", // v1.14.1
		},
	},
	// {

	// 	owner: "elastic",
	// 	name:  "beats", // uses zap v1.7.1
	// 	revs: []string{
	// 		"c94d4bf54ae4884deba1a50e76ee3457c871f431",
	// 		"30311f6ccf9ca7eb47625d6fce67010de220f98a",
	// 		"bc8123abbfde00c235c3daef3caead78ecd2289e",
	// 	},
	// },
	{

		owner: "influxdata",
		name:  "influxdb", //  uses zap v1.9.1
		revs: []string{
			"369186d339389cfd79ab3e303687521d640c0ba2",
			"0f09f4d2ab27fbef88cb49d890d9bec6ee6ca675",
			"7be7328c3bd307f779c69c809b24d30808f72b12",
		},
	},
	// {

	// 	owner: "istio",
	// 	name:  "istio", // uses zap v1.10.0
	// 	revs: []string{
	// 		"4cfbfecfe6aa8d6839cd94e461af4df521393abc",
	// 		"447e561b93723d103fedfa267345f0b8c0134d01",
	// 		"e37e1dc9b52e6dbc2c17712052189964a5493bcb",
	// 	},
	// },
	{

		owner: "distributedio",
		name:  "titan", //  uses zap v1.12.0
		revs: []string{
			"0ad2e75d529bda74472a1dbb5e488ec095b07fe7",
			"33623cc32f8d9f999fd69189d29124d4368c20ab",
			"aef232fbec9089d4468ff06705a3a7f84ee50ea6",
		},
	},
	{

		owner: "etcd-io",
		name:  "etcd", // uses zap v1.14.1
		revs: []string{
			"dfb0a405096af39e694a501de5b0a46962b3050e",
			"1044a8b07c56f3d32a1f3fe91c8ec849a8b17b5e",
			"fb77f9b1d56391318823c434f586ffe371750321",
		},
	},
	{

		owner: "pingcap",
		name:  "tidb", // uses zap v1.14.1
		revs: []string{
			"b4f42abc36d893ec3f443af78fc62705a2e54236",
			"43764a59b7dcb846dc1e9754e8f125818c69a96f",
			"2f9a487ebbd2f1a46b5f2c2262ae8f0ef4c4d42f",
		},
	},
}

func main() {
	if err := mainErr(); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}

func mainErr() error {
	for _, dir := range []string{"repos", "indexes"} {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}
	}

	phases := []func() error{
		func() error { return ensureAllCloned() },
		func() error { return ensureAllIndexed() },
		func() error { return ensureAllUploaded() },
	}

	for _, phase := range phases {
		if err := phase(); err != nil {
			return err
		}
	}
	return nil
}

func ensureAllCloned() error {
	return parallelForRepo(ensureCloned)
}

func ensureCloned(repo Repo) error {
	if _, err := os.Stat(filepath.Join("repos", repo.name)); err == nil || !os.IsNotExist(err) {
		return err
	}

	fmt.Printf("Cloning %s/%s\n", repo.owner, repo.name)
	cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", repo.owner, repo.name)
	return runCommand("repos", "git", "clone", cloneURL)
}

func ensureAllIndexed() error {
	return parallelForRepo(ensureIndexed)
}

func ensureIndexed(repo Repo) error {
	errs := make(chan error, len(repo.revs))

	var wg sync.WaitGroup
	for _, rev := range repo.revs {
		wg.Add(1)

		go func(rev string) {
			defer wg.Done()
			errs <- index(repo.owner, repo.name, rev)
		}(rev)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func index(owner, name, rev string) error {
	indexFile, err := filepath.Abs(filepath.Join("indexes", fmt.Sprintf("%s.%s.dump", name, rev)))
	if err != nil {
		return err
	}

	if _, err := os.Stat(indexFile); err == nil || !os.IsNotExist(err) {
		return err
	}

	tempDir, err := ioutil.TempDir(".", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join("repos", name)
	targetDir := filepath.Join(tempDir, name)

	commands := []func() error{
		func() error { return runCommand("", "cp", "-r", sourceDir, tempDir) },
		func() error { return runCommand(targetDir, "git", "checkout", rev) },
		func() error { return runCommand(targetDir, "go", "mod", "vendor") },
		func() error { return runCommand(targetDir, "lsif-go", "-o", indexFile) },
	}

	fmt.Printf("Indexing %s/%s at %s\n", owner, name, rev)

	for _, command := range commands {
		if err := command(); err != nil {
			return err
		}
	}
	return nil
}

func ensureAllUploaded() error {
	total := 0
	for _, repo := range repos {
		total += len(repo.revs)
	}
	errs := make(chan error, total)

	var wg sync.WaitGroup
	for _, repo := range repos {
		for _, rev := range repo.revs {
			wg.Add(1)

			go func(repo Repo, rev string) {
				defer wg.Done()
				errs <- upload(repo.owner, repo.name, rev)
			}(repo, rev)
		}
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func upload(owner, name, rev string) error {
	fmt.Printf("Uploading %s/%s at %s\n", owner, name, rev)

	if err := runCommand(
		"indexes",
		"src",
		"-endpoint=http://localhost:3080",
		"lsif",
		"upload",
		"-root=/",
		fmt.Sprintf("-repo=%s", fmt.Sprintf("github.com/%s/%s", owner, name)),
		fmt.Sprintf("-commit=%s", rev),
		fmt.Sprintf("-file=%s", filepath.Join(fmt.Sprintf("%s.%s.dump", name, rev))),
	); err != nil {
		return err
	}

	return nil
}

func parallelForRepo(f func(repo Repo) error) error {
	errs := make(chan error, len(repos))

	var wg sync.WaitGroup
	for _, repo := range repos {
		wg.Add(1)

		go func(repo Repo) {
			defer wg.Done()
			errs <- f(repo)
		}(repo)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func runCommand(dir, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir

	if output, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrap(err, fmt.Sprintf("error running '%s':\n%s\n", strings.Join(append([]string{command}, args...), " "), output))
	}

	return nil
}
