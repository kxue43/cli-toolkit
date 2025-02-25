# Personal CLI Toolkit

The Python package name is `cli-toolkit`. Its exposed CLI binary is named `toolkit`.

Example invocation is:

```bash
toolkit [OPTIONS] COMMAND [ARGS]
```

## Setup Project

Install `pyenv` and `poetry>=2.0.1` first.

```bash
pyenv local 3.9
poetry env use $(pyenv which python)
poetry install
```

## Make a Release

```bash
eval $(poetry env activate)
python -m build
export TOOLKIT_VERSION=$(poetry version --short)
gh release create $TOOLKIT_VERSION --latest dist/cli_toolkit-${TOOLKIT_VERSION}-py3-none-any.whl
```

## Install from GitHub Release Asset

Users of this package can conveniently install it from the wheel file available as GitHub release asset.
This package is not on PyPI.

```bash
pip install "cli-toolkit@https://github.com/kxue43/cli-toolkit/releases/download/1.1.0/cli_toolkit-1.1.0-py3-none-any.whl"
```
