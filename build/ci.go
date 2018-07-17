// +build none

/*
The ci cmd is called from Continuous Integration scripts.

Usage: go run ci.go <cmd> <cmd flags/arguments>

Available commands are:

   install    [-arch architecture] [ packages... ]                                           -- builds packages and executables
   test       [ -coverage ] [ -vet ] [ packages... ]                                         -- runs the tests
   archive    [-arch architecture] [ -type zip|tar ] [ -signer key-envvar ] [ -upload dest ] -- archives build artefacts
   importkeys                                                                                -- imports signing keys from env
   debsrc     [ -signer key-id ] [ -upload dest ]                                            -- creates a debian source package
   nsis                                                                                      -- creates a Windows NSIS installer
   aar        [ -sign key-id ] [-deploy repo] [ -upload dest ]                               -- creates an Android archive
   xcode      [ -sign key-id ] [-deploy repo] [ -upload dest ]                               -- creates an iOS XCode framework
   xgo        [ options ]                                                                    -- cross builds according to options

For all commands, -n prevents execution of external programs (dry run mode).

*/
package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/internal/buildtool"
)

var (
	// Files that end up in the siotchain*.zip archive.
	siotchainArchiveFiles = []string{
		"COPYING",
		executablePath("siotchain"),
	}

	// Files that end up in the siotchain-alltools*.zip archive.
	allToolsArchiveFiles = []string{
		"COPYING",
		executablePath("siotchain"),
		executablePath("siotchain_cli"),
	}

	// A debian package is created for all executables listed here.
	debExecutables = []debExecutable{
		{
			Name:        "siotchain",
			Description: "Siotchain node.",
		},
		{
			Name:        "siotchain_cli",
			Description: "Siotchain CLI client.",
		},
	}

	// Distros for which packages are created.
	// Note: vivid is unsupported because there is no golang-1.6 package for it.
	debDistros = []string{"trusty", "wily", "xenial", "yakkety"}
)

var GOBIN, _ = filepath.Abs(filepath.Join("build", "bin"))

func executablePath(name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(GOBIN, name)
}

func main() {
	log.SetFlags(log.Lshortfile)

	if _, err := os.Stat(filepath.Join("build", "ci.go")); os.IsNotExist(err) {
		log.Fatal("this script must be run from the root of the repository")
	}
	if len(os.Args) < 2 {
		log.Fatal("need subcommand as first argument")
	}
	switch os.Args[1] {
	case "install":
		doInstall(os.Args[2:])
	case "test":
		doTest(os.Args[2:])
	case "archive":
		doArchive(os.Args[2:])
	case "debsrc":
		doDebianSource(os.Args[2:])
	case "nsis":
		doWindowsInstaller(os.Args[2:])
	case "aar":
		doAndroidArchive(os.Args[2:])
	case "xcode":
		doXCodeFramework(os.Args[2:])
	case "xgo":
		doXgo(os.Args[2:])
	default:
		log.Fatal("unknown cmd ", os.Args[1])
	}
}

// Compiling

func doInstall(cmdline []string) {
	var (
		arch = flag.String("arch", "", "Architecture to cross build for")
	)
	flag.CommandLine.Parse(cmdline)
	env := buildtool.Env()

	// Compile packages given as arguments, or everything if there are no arguments.
	packages := []string{"./..."}
	if flag.NArg() > 0 {
		packages = flag.Args()
	}
	if *arch == "" || *arch == runtime.GOARCH {
		goinstall := goTool("install", buildFlags(env)...)
		goinstall.Args = append(goinstall.Args, "-v")
		goinstall.Args = append(goinstall.Args, packages...)
		buildtool.MustRun(goinstall)
		return
	}
	// If we are cross compiling to ARMv5 ARMv6 or ARMv7, clean any prvious builds
	if *arch == "arm" {
		os.RemoveAll(filepath.Join(runtime.GOROOT(), "pkg", runtime.GOOS+"_arm"))
		for _, path := range filepath.SplitList(buildtool.GOPATH()) {
			os.RemoveAll(filepath.Join(path, "pkg", runtime.GOOS+"_arm"))
		}
	}
	// Seems we are cross compiling, work around forbidden GOBIN
	goinstall := goToolArch(*arch, "install", buildFlags(env)...)
	goinstall.Args = append(goinstall.Args, "-v")
	goinstall.Args = append(goinstall.Args, []string{"-buildmode", "archive"}...)
	goinstall.Args = append(goinstall.Args, packages...)
	buildtool.MustRun(goinstall)

	if cmds, err := ioutil.ReadDir("client"); err == nil {
		for _, cmd := range cmds {
			pkgs, err := parser.ParseDir(token.NewFileSet(), filepath.Join(".", "client", cmd.Name()), nil, parser.PackageClauseOnly)
			if err != nil {
				log.Fatal(err)
			}
			for name, _ := range pkgs {
				if name == "main" {
					gobuild := goToolArch(*arch, "build", buildFlags(env)...)
					gobuild.Args = append(gobuild.Args, "-v")
					gobuild.Args = append(gobuild.Args, []string{"-o", executablePath(cmd.Name())}...)
					gobuild.Args = append(gobuild.Args, "."+string(filepath.Separator)+filepath.Join("client", cmd.Name()))
					buildtool.MustRun(gobuild)
					break
				}
			}
		}
	}
}

func buildFlags(env buildtool.Environment) (flags []string) {
	if os.Getenv("GO_OPENCL") != "" {
		flags = append(flags, "-tags", "opencl")
	}

	// Since Go 1.5, the separator char for link time assignments
	// is '=' and using ' ' prints a warning. However, Go < 1.5 does
	// not support using '='.
	sep := "="
	if runtime.Version() > "go1.5" || strings.Contains(runtime.Version(), "devel") {
		sep = "="
	}
	// Set gitCommit constant via link-time assignment.
	if env.Commit != "" {
		flags = append(flags, "-ldflags", "-X main.gitCommit"+sep+env.Commit)
	}
	return flags
}

func goTool(subcmd string, args ...string) *exec.Cmd {
	return goToolArch(runtime.GOARCH, subcmd, args...)
}

func goToolArch(arch string, subcmd string, args ...string) *exec.Cmd {
	gocmd := filepath.Join(runtime.GOROOT(), "bin", "go")
	cmd := exec.Command(gocmd, subcmd)
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = []string{
		"GO15VENDOREXPERIMENT=1",
		"GOPATH=" + buildtool.GOPATH(),
	}
	if arch == "" || arch == runtime.GOARCH {
		cmd.Env = append(cmd.Env, "GOBIN="+GOBIN)
	} else {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=1")
		cmd.Env = append(cmd.Env, "GOARCH="+arch)
	}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GOPATH=") || strings.HasPrefix(e, "GOBIN=") {
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	return cmd
}

// Running The Tests
//
// "tests" also includes static analysis tools such as vet.

func doTest(cmdline []string) {
	var (
		vet      = flag.Bool("vet", false, "Whether to run go vet")
		coverage = flag.Bool("coverage", false, "Whether to record code coverage")
	)
	flag.CommandLine.Parse(cmdline)

	packages := []string{"./..."}
	if len(flag.CommandLine.Args()) > 0 {
		packages = flag.CommandLine.Args()
	}
	if len(packages) == 1 && packages[0] == "./..." {
		// Resolve ./... manually since go vet will fail on vendored stuff
		out, err := goTool("list", "./...").CombinedOutput()
		if err != nil {
			log.Fatalf("package listing failed: %v\n%s", err, string(out))
		}
		packages = []string{}
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.Contains(line, "vendor") {
				packages = append(packages, strings.TrimSpace(line))
			}
		}
	}
	// Run analysis tools before the tests.
	if *vet {
		buildtool.MustRun(goTool("vet", packages...))
	}

	// Run the actual tests.
	gotest := goTool("test")
	// Test a single package at a time. CI builders are slow
	// and some tests run into timeouts under load.
	gotest.Args = append(gotest.Args, "-p", "1")
	if *coverage {
		gotest.Args = append(gotest.Args, "-covermode=atomic", "-cover")
	}
	gotest.Args = append(gotest.Args, packages...)
	buildtool.MustRun(gotest)
}

// Release Packaging

func doArchive(cmdline []string) {
	var (
		arch   = flag.String("arch", runtime.GOARCH, "Architecture cross packaging")
		atype  = flag.String("type", "zip", "Type of archive to write (zip|tar)")
		signer = flag.String("signer", "", `Environment variable holding the signing key (e.g. LINUX_SIGNING_KEY)`)
		upload = flag.String("upload", "", `Destination to upload the archives (usually "siotchainstore/builds")`)
		ext    string
	)
	flag.CommandLine.Parse(cmdline)
	switch *atype {
	case "zip":
		ext = ".zip"
	case "tar":
		ext = ".tar.gz"
	default:
		log.Fatal("unknown archive type: ", atype)
	}

	var (
		env      = buildtool.Env()
		base     = archiveBasename(*arch, env)
		siotchain     = "siotchain-" + base + ext
		alltools = "siotchain-alltools-" + base + ext
	)
	maybeSkipArchive(env)
	if err := buildtool.WriteArchive(siotchain, siotchainArchiveFiles); err != nil {
		log.Fatal(err)
	}
	if err := buildtool.WriteArchive(alltools, allToolsArchiveFiles); err != nil {
		log.Fatal(err)
	}
	for _, archive := range []string{siotchain, alltools} {
		if err := archiveUpload(archive, *upload, *signer); err != nil {
			log.Fatal(err)
		}
	}
}

func archiveBasename(arch string, env buildtool.Environment) string {
	platform := runtime.GOOS + "-" + arch
	if arch == "arm" {
		platform += os.Getenv("GOARM")
	}
	if arch == "android" {
		platform = "android-all"
	}
	if arch == "ios" {
		platform = "ios-all"
	}
	return platform + "-" + archiveVersion(env)
}

func archiveVersion(env buildtool.Environment) string {
	version := buildtool.VERSION()
	if isUnstableBuild(env) {
		version += "-unstable"
	}
	if env.Commit != "" {
		version += "-" + env.Commit[:8]
	}
	return version
}

func archiveUpload(archive string, blobstore string, signer string) error {
	// If signing was requested, generate the signature files
	if signer != "" {
		pgpkey, err := base64.StdEncoding.DecodeString(os.Getenv(signer))
		if err != nil {
			return fmt.Errorf("invalid base64 %s", signer)
		}
		if err := buildtool.PGPSignFile(archive, archive+".asc", string(pgpkey)); err != nil {
			return err
		}
	}
	// If uploading to Azure was requested, push the archive possibly with its signature
	if blobstore != "" {
		auth := buildtool.AzureBlobstoreConfig{
			Account:   strings.Split(blobstore, "/")[0],
			Token:     os.Getenv("AZURE_BLOBSTORE_TOKEN"),
			Container: strings.SplitN(blobstore, "/", 2)[1],
		}
		if err := buildtool.AzureBlobstoreUpload(archive, filepath.Base(archive), auth); err != nil {
			return err
		}
		if signer != "" {
			if err := buildtool.AzureBlobstoreUpload(archive+".asc", filepath.Base(archive+".asc"), auth); err != nil {
				return err
			}
		}
	}
	return nil
}

// skips archiving for some build configurations.
func maybeSkipArchive(env buildtool.Environment) {
	if env.IsPullRequest {
		log.Printf("skipping because this is a PR build")
		os.Exit(0)
	}
	if env.Branch != "master" && !strings.HasPrefix(env.Tag, "v1.") {
		log.Printf("skipping because branch %q, tag %q is not on the whitelist", env.Branch, env.Tag)
		os.Exit(0)
	}
}

// Debian Packaging

func doDebianSource(cmdline []string) {
	var (
		signer  = flag.String("signer", "", `Signing key name, also used as package author`)
		upload  = flag.String("upload", "", `Where to upload the source package (usually "ppa:siotchain/siotchain")`)
		workdir = flag.String("workdir", "", `Output directory for packages (uses temp dir if unset)`)
		now     = time.Now()
	)
	flag.CommandLine.Parse(cmdline)
	*workdir = makeWorkdir(*workdir)
	env := buildtool.Env()
	maybeSkipArchive(env)

	// Import the signing key.
	if b64key := os.Getenv("PPA_SIGNING_KEY"); b64key != "" {
		key, err := base64.StdEncoding.DecodeString(b64key)
		if err != nil {
			log.Fatal("invalid base64 PPA_SIGNING_KEY")
		}
		gpg := exec.Command("gpg", "--import")
		gpg.Stdin = bytes.NewReader(key)
		buildtool.MustRun(gpg)
	}

	// Create the packages.
	for _, distro := range debDistros {
		meta := newDebMetadata(distro, *signer, env, now)
		pkgdir := stageDebianSource(*workdir, meta)
		debuild := exec.Command("debuild", "-S", "-sa", "-us", "-uc")
		debuild.Dir = pkgdir
		buildtool.MustRun(debuild)

		changes := fmt.Sprintf("%s_%s_source.changes", meta.Name(), meta.VersionString())
		changes = filepath.Join(*workdir, changes)
		if *signer != "" {
			buildtool.MustRunCommand("debsign", changes)
		}
		if *upload != "" {
			buildtool.MustRunCommand("dput", *upload, changes)
		}
	}
}

func makeWorkdir(wdflag string) string {
	var err error
	if wdflag != "" {
		err = os.MkdirAll(wdflag, 0744)
	} else {
		wdflag, err = ioutil.TempDir("", "siotchain-build-")
	}
	if err != nil {
		log.Fatal(err)
	}
	return wdflag
}

func isUnstableBuild(env buildtool.Environment) bool {
	if env.Branch != "master" && env.Tag != "" {
		return false
	}
	return true
}

type debMetadata struct {
	Env buildtool.Environment

	// siotchain version being built. Note that this
	// is not the debian package version. The package version
	// is constructed by VersionString.
	Version string

	Author       string // "name <email>", also selects signing key
	Distro, Time string
	Executables  []debExecutable
}

type debExecutable struct {
	Name, Description string
}

func newDebMetadata(distro, author string, env buildtool.Environment, t time.Time) debMetadata {
	if author == "" {
		// No signing key, use default author.
		author = "siotchain team"
	}
	return debMetadata{
		Env:         env,
		Author:      author,
		Distro:      distro,
		Version:     buildtool.VERSION(),
		Time:        t.Format(time.RFC1123Z),
		Executables: debExecutables,
	}
}

// Name returns the name of the metapackage that depends
// on all executable packages.
func (meta debMetadata) Name() string {
	if isUnstableBuild(meta.Env) {
		return "siotchain-unstable"
	}
	return "siotchain"
}

// VersionString returns the debian version of the packages.
func (meta debMetadata) VersionString() string {
	vsn := meta.Version
	if meta.Env.Buildnum != "" {
		vsn += "+build" + meta.Env.Buildnum
	}
	if meta.Distro != "" {
		vsn += "+" + meta.Distro
	}
	return vsn
}

// ExeList returns the list of all executable packages.
func (meta debMetadata) ExeList() string {
	names := make([]string, len(meta.Executables))
	for i, e := range meta.Executables {
		names[i] = meta.ExeName(e)
	}
	return strings.Join(names, ", ")
}

// ExeName returns the package name of an executable package.
func (meta debMetadata) ExeName(exe debExecutable) string {
	if isUnstableBuild(meta.Env) {
		return exe.Name + "-unstable"
	}
	return exe.Name
}

// ExeConflicts returns the content of the Conflicts field
// for executable packages.
func (meta debMetadata) ExeConflicts(exe debExecutable) string {
	if isUnstableBuild(meta.Env) {
		// Set up the conflicts list so that the *-unstable packages
		// cannot be installed alongside the regular version.
		//
		// https://www.debian.org/doc/debian-policy/ch-relationships.html
		// is very explicit about Conflicts: and says that Breaks: should
		// be preferred and the conflicting files should be handled via
		// alternates. We might do this eventually but using a conflict is
		// easier now.
		return "siotchain, " + exe.Name
	}
	return ""
}

func stageDebianSource(tmpdir string, meta debMetadata) (pkgdir string) {
	pkg := meta.Name() + "-" + meta.VersionString()
	pkgdir = filepath.Join(tmpdir, pkg)
	if err := os.Mkdir(pkgdir, 0755); err != nil {
		log.Fatal(err)
	}

	// Copy the source code.
	buildtool.MustRunCommand("git", "checkout-index", "-a", "--prefix", pkgdir+string(filepath.Separator))

	// Put the debian build files in place.
	debian := filepath.Join(pkgdir, "debian")
	buildtool.Render("build/deb.rules", filepath.Join(debian, "rules"), 0755, meta)
	buildtool.Render("build/deb.changelog", filepath.Join(debian, "changelog"), 0644, meta)
	buildtool.Render("build/deb.control", filepath.Join(debian, "control"), 0644, meta)
	buildtool.Render("build/deb.copyright", filepath.Join(debian, "copyright"), 0644, meta)
	buildtool.RenderString("8\n", filepath.Join(debian, "compat"), 0644, meta)
	buildtool.RenderString("3.0 (native)\n", filepath.Join(debian, "source/format"), 0644, meta)
	for _, exe := range meta.Executables {
		install := filepath.Join(debian, meta.ExeName(exe)+".install")
		docs := filepath.Join(debian, meta.ExeName(exe)+".docs")
		buildtool.Render("build/deb.install", install, 0644, exe)
		buildtool.Render("build/deb.docs", docs, 0644, exe)
	}

	return pkgdir
}

// Windows installer

func doWindowsInstaller(cmdline []string) {
	// Parse the flags and make skip installer generation on PRs
	var (
		arch    = flag.String("arch", runtime.GOARCH, "Architecture for cross build packaging")
		signer  = flag.String("signer", "", `Environment variable holding the signing key (e.g. WINDOWS_SIGNING_KEY)`)
		upload  = flag.String("upload", "", `Destination to upload the archives (usually "siotchainstore/builds")`)
		workdir = flag.String("workdir", "", `Output directory for packages (uses temp dir if unset)`)
	)
	flag.CommandLine.Parse(cmdline)
	*workdir = makeWorkdir(*workdir)
	env := buildtool.Env()
	maybeSkipArchive(env)

	// Aggregate binaries that are included in the installer
	var (
		devTools []string
		allTools []string
		siotchainTool string
	)
	for _, file := range allToolsArchiveFiles {
		if file == "COPYING" { // license, copied later
			continue
		}
		allTools = append(allTools, filepath.Base(file))
		if filepath.Base(file) == "siotchain.exe" {
			siotchainTool = file
		} else {
			devTools = append(devTools, file)
		}
	}

	// Render NSIS scripts: Installer NSIS contains two installer sections,
	// first section contains the siotchain binary, second section holds the dev tools.
	templateData := map[string]interface{}{
		"License":  "COPYING",
		"Siotchain": siotchainTool,
		"DevTools": devTools,
	}
	buildtool.Render("build/nsis.siotchain.nsi", filepath.Join(*workdir, "siotchain.nsi"), 0644, nil)
	buildtool.Render("build/nsis.install.nsh", filepath.Join(*workdir, "install.nsh"), 0644, templateData)
	buildtool.Render("build/nsis.uninstall.nsh", filepath.Join(*workdir, "uninstall.nsh"), 0644, allTools)
	buildtool.Render("build/nsis.envvarupdate.nsh", filepath.Join(*workdir, "EnvVarUpdate.nsh"), 0644, nil)
	buildtool.CopyFile(filepath.Join(*workdir, "SimpleFC.dll"), "build/nsis.simplefc.dll", 0755)
	buildtool.CopyFile(filepath.Join(*workdir, "COPYING"), "COPYING", 0755)

	// Build the installer. This assumes that all the needed files have been previously
	// built (don't mix building and packaging to keep cross compilation complexity to a
	// minimum).
	version := strings.Split(buildtool.VERSION(), ".")
	if env.Commit != "" {
		version[2] += "-" + env.Commit[:8]
	}
	installer, _ := filepath.Abs("siotchain-" + archiveBasename(*arch, env) + ".exe")
	buildtool.MustRunCommand("makensis.exe",
		"/DOUTPUTFILE="+installer,
		"/DMAJORVERSION="+version[0],
		"/DMINORVERSION="+version[1],
		"/DBUILDVERSION="+version[2],
		"/DARCH="+*arch,
		filepath.Join(*workdir, "siotchain.nsi"),
	)

	// Sign and publish installer.
	if err := archiveUpload(installer, *upload, *signer); err != nil {
		log.Fatal(err)
	}
}

// Android archives

func doAndroidArchive(cmdline []string) {
	var (
		signer = flag.String("signer", "", `Environment variable holding the signing key (e.g. ANDROID_SIGNING_KEY)`)
		deploy = flag.String("deploy", "", `Destination to deploy the archive (usually "https://oss.sonatype.org")`)
		upload = flag.String("upload", "", `Destination to upload the archive (usually "siotchainstore/builds")`)
	)
	flag.CommandLine.Parse(cmdline)
	env := buildtool.Env()

	// Build the Android archive and Maven resources
	buildtool.MustRun(goTool("get", "golang.org/x/mobile/cmd/gomobile"))
	buildtool.MustRun(gomobileTool("init"))
	buildtool.MustRun(gomobileTool("bind", "--target", "android", "--javapkg", "org.siotchain", "-v", "github.com/ethereum/go-ethereum/mobile"))

	meta := newMavenMetadata(env)
	buildtool.Render("build/mvn.pom", meta.Package+".pom", 0755, meta)

	// Skip Maven deploy and Azure upload for PR builds
	maybeSkipArchive(env)

	// Sign and upload the archive to Azure
	archive := "siotchain-" + archiveBasename("android", env) + ".aar"
	os.Rename("siotchain.aar", archive)

	if err := archiveUpload(archive, *upload, *signer); err != nil {
		log.Fatal(err)
	}
	// Sign and upload all the artifacts to Maven Central
	os.Rename(archive, meta.Package+".aar")
	if *signer != "" && *deploy != "" {
		// Import the signing key into the local GPG instance
		if b64key := os.Getenv(*signer); b64key != "" {
			key, err := base64.StdEncoding.DecodeString(b64key)
			if err != nil {
				log.Fatalf("invalid base64 %s", *signer)
			}
			gpg := exec.Command("gpg", "--import")
			gpg.Stdin = bytes.NewReader(key)
			buildtool.MustRun(gpg)
		}
		// Upload the artifacts to Sonatype and/or Maven Central
		repo := *deploy + "/service/local/staging/deploy/maven2"
		if meta.Develop {
			repo = *deploy + "/content/repositories/snapshots"
		}
		buildtool.MustRunCommand("mvn", "gpg:sign-and-deploy-file",
			"-settings=build/mvn.settings", "-Durl="+repo, "-DrepositoryId=ossrh",
			"-DpomFile="+meta.Package+".pom", "-Dfile="+meta.Package+".aar")
	}
}

func gomobileTool(subcmd string, args ...string) *exec.Cmd {
	cmd := exec.Command(filepath.Join(GOBIN, "gomobile"), subcmd)
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = []string{
		"GOPATH=" + buildtool.GOPATH(),
	}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GOPATH=") {
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	return cmd
}

type mavenMetadata struct {
	Version      string
	Package      string
	Develop      bool
	Contributors []mavenContributor
}

type mavenContributor struct {
	Name  string
	Email string
}

func newMavenMetadata(env buildtool.Environment) mavenMetadata {
	// Collect the list of authors from the repo root
	contribs := []mavenContributor{}
	if authors, err := os.Open("AUTHORS"); err == nil {
		defer authors.Close()

		scanner := bufio.NewScanner(authors)
		for scanner.Scan() {
			// Skip any whitespace from the authors list
			line := strings.TrimSpace(scanner.Text())
			if line == "" || line[0] == '#' {
				continue
			}
			// Split the author and insert as a contributor
			re := regexp.MustCompile("([^<]+) <(.+>)")
			parts := re.FindStringSubmatch(line)
			if len(parts) == 3 {
				contribs = append(contribs, mavenContributor{Name: parts[1], Email: parts[2]})
			}
		}
	}
	// Render the version and package strings
	version := buildtool.VERSION()
	if isUnstableBuild(env) {
		version += "-SNAPSHOT"
	}
	return mavenMetadata{
		Version:      version,
		Package:      "siotchain-" + version,
		Develop:      isUnstableBuild(env),
		Contributors: contribs,
	}
}

// XCode frameworks

func doXCodeFramework(cmdline []string) {
	var (
		signer = flag.String("signer", "", `Environment variable holding the signing key (e.g. IOS_SIGNING_KEY)`)
		deploy = flag.String("deploy", "", `Destination to deploy the archive (usually "trunk")`)
		upload = flag.String("upload", "", `Destination to upload the archives (usually "siotchainstore/builds")`)
	)
	flag.CommandLine.Parse(cmdline)
	env := buildtool.Env()

	// Build the iOS XCode framework
	buildtool.MustRun(goTool("get", "golang.org/x/mobile/cmd/gomobile"))
	buildtool.MustRun(gomobileTool("init"))

	archive := "siotchain-" + archiveBasename("ios", env)
	if err := os.Mkdir(archive, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	bind := gomobileTool("bind", "--target", "ios", "--tags", "ios", "--prefix", "GE", "-v", "github.com/ethereum/go-ethereum/mobile")
	bind.Dir, _ = filepath.Abs(archive)
	buildtool.MustRun(bind)
	buildtool.MustRunCommand("tar", "-zcvf", archive+".tar.gz", archive)

	// Skip CocoaPods deploy and Azure upload for PR builds
	maybeSkipArchive(env)

	// Sign and upload the framework to Azure
	if err := archiveUpload(archive+".tar.gz", *upload, *signer); err != nil {
		log.Fatal(err)
	}
	// Prepare and upload a PodSpec to CocoaPods
	if *deploy != "" {
		meta := newPodMetadata(env)
		buildtool.Render("build/pod.podspec", meta.Name+".podspec", 0755, meta)
		buildtool.MustRunCommand("pod", *deploy, "push", meta.Name+".podspec", "--allow-warnings")
	}
}

type podMetadata struct {
	Name         string
	Version      string
	Commit       string
	Contributors []podContributor
}

type podContributor struct {
	Name  string
	Email string
}

func newPodMetadata(env buildtool.Environment) podMetadata {
	// Collect the list of authors from the repo root
	contribs := []podContributor{}
	if authors, err := os.Open("AUTHORS"); err == nil {
		defer authors.Close()

		scanner := bufio.NewScanner(authors)
		for scanner.Scan() {
			// Skip any whitespace from the authors list
			line := strings.TrimSpace(scanner.Text())
			if line == "" || line[0] == '#' {
				continue
			}
			// Split the author and insert as a contributor
			re := regexp.MustCompile("([^<]+) <(.+>)")
			parts := re.FindStringSubmatch(line)
			if len(parts) == 3 {
				contribs = append(contribs, podContributor{Name: parts[1], Email: parts[2]})
			}
		}
	}
	name := "Siotchain"
	if isUnstableBuild(env) {
		name += "Develop"
	}
	return podMetadata{
		Name:         name,
		Version:      archiveVersion(env),
		Commit:       env.Commit,
		Contributors: contribs,
	}
}

// Cross compilation

func doXgo(cmdline []string) {
	flag.CommandLine.Parse(cmdline)
	env := buildtool.Env()

	// Make sure xgo is available for cross compilation
	gogetxgo := goTool("get", "github.com/karalabe/xgo")
	buildtool.MustRun(gogetxgo)

	// Execute the actual cross compilation
	xgo := xgoTool(append(buildFlags(env), flag.Args()...))
	buildtool.MustRun(xgo)
}

func xgoTool(args []string) *exec.Cmd {
	cmd := exec.Command(filepath.Join(GOBIN, "xgo"), args...)
	cmd.Env = []string{
		"GOPATH=" + buildtool.GOPATH(),
		"GOBIN=" + GOBIN,
	}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GOPATH=") || strings.HasPrefix(e, "GOBIN=") {
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	return cmd
}
