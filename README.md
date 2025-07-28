# Personal CLI Toolkit

Various CLI programs that aid my own workflows.

## Installation

`toolkit` provides starter templates for Go, Python or AWS CDK projects.

```bash
go install github.com/kxue43/cli-toolkit/cmd/toolkit@latest
```

`toolkit-assume-role` performs the AWS CLI credential process.
It only works on macOS and Linux because it needs to read and write `/dev/tty`.

```bash
go install github.com/kxue43/cli-toolkit/cmd/toolkit-assume-role@latest
```

`toolkit-show-md` takes a Markdown file, converts it to a GitHub style HTML and
displays the HTML in user's default browser. It's used by
[kxue43/showmd-vim-plugin](https://github.com/kxue43/showmd-vim-plugin).
