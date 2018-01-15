package graphqlbackend

import (
	"context"
	"errors"
	"os"
	"path"
	"strings"
	"time"

	"sourcegraph.com/sourcegraph/sourcegraph/pkg/db"
	"sourcegraph.com/sourcegraph/sourcegraph/pkg/vcs"
)

type treeResolver struct {
	commit *gitCommitResolver

	path    string
	entries []os.FileInfo
}

func makeTreeResolver(ctx context.Context, commit *gitCommitResolver, path string, recursive bool) (*treeResolver, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	vcsrepo, err := db.RepoVCS.Open(ctx, commit.repositoryIDInt32())
	if err != nil {
		return nil, err
	}

	if recursive && path != "" {
		return nil, errors.New("not implemented")
	}

	entries, err := vcsrepo.ReadDir(ctx, vcs.CommitID(commit.oid), path, recursive)
	if err != nil {
		if strings.Contains(err.Error(), "file does not exist") { // TODO proper error value
			// empty tree is not an error
		} else {
			return nil, err
		}
	}

	return &treeResolver{
		commit:  commit,
		path:    path,
		entries: entries,
	}, nil
}

func (r *treeResolver) toFileResolvers(filter func(fi os.FileInfo) bool) []*fileResolver {
	var l []*fileResolver
	for _, entry := range r.entries {
		if filter == nil || filter(entry) {
			l = append(l, &fileResolver{
				commit: r.commit,
				name:   entry.Name(),
				path:   path.Join(r.path, entry.Name()),
				stat:   entry,
			})
		}
	}
	return l
}

func (r *treeResolver) Entries() []*fileResolver {
	return r.toFileResolvers(nil)
}

func (r *treeResolver) Directories() []*fileResolver {
	return r.toFileResolvers(func(fi os.FileInfo) bool {
		return fi.Mode().IsDir()
	})
}

func (r *treeResolver) Files() []*fileResolver {
	return r.toFileResolvers(func(fi os.FileInfo) bool {
		return !fi.Mode().IsDir()
	})
}

func (r *fileResolver) Tree(ctx context.Context) (*treeResolver, error) {
	return makeTreeResolver(ctx, r.commit, r.path, false)
}
