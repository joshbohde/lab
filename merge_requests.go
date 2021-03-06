package lab

import (
	"errors"
	"fmt"
	"io"
)

type MergeRequest struct {
	Body
	URL          string
	SourceBranch string
	TargetBranch string
	KeepSource   bool
}

type CreateMergeRequestOptions struct {
	Message       string
	File          string
	Edit          bool
	SourceBranch  string
	TargetBranch  string
	KeepSource    bool
	OpenInBrowser bool
}

func (opts *CreateMergeRequestOptions) MergeRequest() MergeRequest {
	ret := MergeRequest{
		SourceBranch: opts.SourceBranch,
		TargetBranch: opts.TargetBranch,
		KeepSource:   opts.KeepSource,
	}
	ret.ParseContent(opts.Message)
	return ret

}

type MergeRequestService struct {
	Git     Git
	Gitlab  Gitlab
	Message Message
	Browser Browser
	Writer  io.Writer
}

func (service *MergeRequestService) Create(opts *CreateMergeRequestOptions) error {
	if opts.SourceBranch == "" {
		localBranch, err := service.Git.LocalBranch()
		if err != nil {
			return err
		}
		opts.SourceBranch = localBranch
	}

	remote, err := service.Git.RemoteProject()
	if err != nil {
		return err
	}

	project, err := service.Gitlab.Project(remote)
	if err != nil {
		return err
	}

	if opts.TargetBranch == "" {
		opts.TargetBranch = project.DefaultBranch
	}

	remoteTarget := fmt.Sprintf("%s/%s", remote.Name, opts.TargetBranch)
	remoteSource := fmt.Sprintf("%s/%s", remote.Name, opts.SourceBranch)
	refs, err := service.Git.RevList(remoteTarget, remoteSource)
	if err != nil {
		return err
	}

	if len(refs) == 0 {
		return fmt.Errorf("%s has no differences to %s, did you forget to push?", remoteSource, remoteTarget)
	}

	if opts.Message == "" {
		msg, err := service.Git.CommitMessage(refs[len(refs)-1])
		if err != nil {
			return err
		}
		opts.Message = msg
	}

	commitMessages, err := service.Git.CommitMessages(remoteTarget, remoteSource)
	if err != nil {
		return fmt.Errorf("Error getting commit messages: %w", err)
	}

	delete, err := service.Message.GetMessage(&opts.Message, MessageOpts{
		Edit:      opts.Edit,
		InputFile: opts.File,
		EditFile:  "MERGE_REQUESTMSG",
		Topic:     "merge request",
		Comment:   fmt.Sprintf("Requesting a merge from %s to %s.\n\nWrite a message for this merge request. The first line is the title and the rest is the description.\n\nCommit list:\n\n%s", opts.SourceBranch, opts.TargetBranch, commitMessages),
	})

	if err != nil {
		return err
	}

	mr := opts.MergeRequest()

	if mr.Title == "" {
		_ = delete()
		return errors.New("merge request title is blank")
	}

	err = service.Gitlab.CreateMergeRequest(remote, &mr)

	if err != nil {
		return err
	}

	if opts.OpenInBrowser {
		err = service.Browser.Open(mr.URL)
		if err != nil {
			return err
		}
	} else {
		fmt.Fprintf(service.Writer, "%s\n", mr.URL)
	}

	err = delete()

	return err
}
