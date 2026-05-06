// SPDX-License-Identifier: Apache-2.0

// Command publish uses the go-gemara bundle SDK (Assemble → Pack) and
// oras.Copy to push a Gemara artifact to an OCI registry. The caller
// (action.yml) MUST run this program from the root file's parent
// directory so relative mapping-reference URLs resolve correctly.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gemaraproj/go-gemara"
	"github.com/gemaraproj/go-gemara/bundle"
	"github.com/gemaraproj/go-gemara/fetcher"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

func main() {
	registry := flag.String("registry", "", "registry host (e.g. ghcr.io)")
	repository := flag.String("repository", "", "repository path without host (e.g. org/bundles/my-policy)")
	tag := flag.String("tag", "", "tag to apply on the remote repository")
	file := flag.String("file", "", "root Gemara YAML file (relative to CWD or absolute)")
	username := flag.String("username", "", "registry username")
	bundleVersion := flag.String("bundle-version", "1", "bundle manifest bundle-version")
	gemaraVersion := flag.String("gemara-version", "", "bundle manifest gemara-version (optional)")
	validate := flag.Bool("validate", true, "run gemara.Load schema validation before assemble")
	flag.Parse()

	password := os.Getenv("GEMARA_REGISTRY_PASSWORD")
	if *registry == "" || *repository == "" || *tag == "" || *file == "" || *username == "" || password == "" {
		flag.Usage()
		fmt.Fprintln(os.Stderr, "GEMARA_REGISTRY_PASSWORD must be set.")
		os.Exit(2)
	}

	data, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read root file: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	if *validate {
		if err := validateRoot(ctx, *file, data); err != nil {
			fmt.Fprintf(os.Stderr, "validate: %v\n", err)
			os.Exit(1)
		}
	}

	// Assemble: walk imports/mapping-references, fetch dependencies.
	// CWD must be the root file's directory for relative url: paths.
	src := bundle.File{Name: filepath.Base(*file), Data: data}
	m := bundle.Manifest{BundleVersion: *bundleVersion, GemaraVersion: *gemaraVersion}
	asm := bundle.NewAssembler(&fetcher.File{})
	b, err := asm.Assemble(ctx, m, src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "assemble: %v\n", err)
		os.Exit(1)
	}

	// Pack into an in-memory OCI store.
	store := memory.New()
	desc, err := bundle.Pack(ctx, store, b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pack: %v\n", err)
		os.Exit(1)
	}

	// Push to remote registry.
	repoRef := fmt.Sprintf("%s/%s", *registry, *repository)
	repo, err := remote.NewRepository(repoRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "remote repo: %v\n", err)
		os.Exit(1)
	}
	repo.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.DefaultCache,
		Credential: auth.StaticCredential(*registry, auth.Credential{
			Username: *username,
			Password: password,
		}),
	}

	if err := store.Tag(ctx, desc, *tag); err != nil {
		fmt.Fprintf(os.Stderr, "tag local store: %v\n", err)
		os.Exit(1)
	}
	if _, err := oras.Copy(ctx, store, *tag, repo, *tag, oras.DefaultCopyOptions); err != nil {
		fmt.Fprintf(os.Stderr, "oras copy: %v\n", err)
		os.Exit(1)
	}

	manifestDesc, err := repo.Resolve(ctx, *tag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve remote tag %q after copy: %v\n", *tag, err)
		os.Exit(1)
	}
	digestStr := manifestDesc.Digest.String()
	if err := writeGitHubOutput("digest", digestStr); err != nil {
		fmt.Fprintf(os.Stderr, "write GITHUB_OUTPUT: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("digest=%s\n", digestStr)
}

func writeGitHubOutput(key, value string) error {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s=%s\n", key, value)
	return err
}

// validateRoot runs gemara.Load schema validation on the root file.
// This catches enum errors (e.g. invalid MethodType) that the assembler's
// structural parse does not check.
func validateRoot(ctx context.Context, filePath string, data []byte) error {
	t, err := gemara.DetectType(data)
	if err != nil {
		return fmt.Errorf("detect type: %w", err)
	}
	f := &fetcher.File{}
	switch t {
	case gemara.PolicyArtifact:
		_, err = gemara.Load[gemara.Policy](ctx, f, filePath)
	case gemara.GuidanceCatalogArtifact:
		_, err = gemara.Load[gemara.GuidanceCatalog](ctx, f, filePath)
	default:
		_, err = gemara.Load[gemara.ControlCatalog](ctx, f, filePath)
	}
	return err
}
