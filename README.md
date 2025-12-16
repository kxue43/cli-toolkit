# Personal CLI Toolkit

Various CLI programs that aid my own workflows.

## Installation

- `toolkit` provides starter templates for Go, Python or AWS CDK projects.

  ```bash
  go install github.com/kxue43/cli-toolkit/cmd/toolkit@latest
  ```

- `toolkit-assume-role` performs the AWS CLI credential process.
  It only works on macOS and Linux because it needs to read and write `/dev/tty`.

  ```bash
  go install github.com/kxue43/cli-toolkit/cmd/toolkit-assume-role@latest
  ```

- `toolkit-show-md` takes a Markdown file, converts it to a GitHub style HTML and displays the HTML in user's default browser.

  ```bash
  go install github.com/kxue43/cli-toolkit/cmd/toolkit-show-md@latest
  ```

- `toolkit-serve-static` provides a local server of static websites. It can be used together with
  [air-verse/air](https://github.com/air-verse/air) to add live-reload capability to the server.

  ```bash
  go install github.com/kxue43/cli-toolkit/cmd/toolkit-serve-static@latest
  ```
