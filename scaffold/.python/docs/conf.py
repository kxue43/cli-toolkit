# Configuration file for the Sphinx documentation builder.
#
# For the full list of built-in configuration values, see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html

# -- Project information -----------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#project-information
from importlib import import_module
from inspect import getsourcelines
from pathlib import Path


project = "{% .ProjectName %}"
copyright = "Public Domain"
author = "Ke Xue"
release = "0.1.0"

# -- General configuration ---------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#general-configuration

extensions = [
    "sphinx.ext.autodoc",
    "sphinx_autodoc_typehints",
    "sphinx.ext.linkcode",
    "sphinx.ext.intersphinx",
    "sphinx_copybutton",
    "sphinx.ext.githubpages",
]

templates_path = ["_templates"]
exclude_patterns = ["_build", "Thumbs.db", ".DS_Store"]

# -- Options for HTML output -------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#options-for-html-output

html_theme = "sphinx_rtd_theme"
html_static_path = ["_static"]
html_css_files = ["custom.css"]

# -- Options for autodoc extension -------------------------------------------------
autodoc_typehints = "none"
autodoc_member_order = "bysource"

# -- Options for sphinx_autodoc_typehints extension ------------------------------------

# -- Options for intersphinx extension -------------------------------------------------
intersphinx_mapping = {
    "python": ("https://docs.python.org/3", None),
}

# -- Options for sphinx-copybutton -------------------------------------------------
copybutton_prompt_text = "$ "
copybutton_line_continuation_character = "\\"
copybutton_here_doc_delimiter = "EOF"


# -- Options for sphinx.ext.linkcode -------------------------------------------------
def linkcode_resolve(domain, info):
    file_dir = Path(__file__).parent
    src_dir = file_dir.parent.joinpath("src")
    if domain != "py":
        return None
    if not info["module"]:
        return None
    if "." in info["fullname"]:
        return None
    filename = info["module"].replace(".", "/")
    if src_dir.joinpath(filename).is_dir():
        filename = f"{filename}/__init__"
    filename = f"src/{filename}"
    module = import_module(info["module"])
    obj = getattr(module, info["fullname"])
    anchor = f"#L{getsourcelines(obj)[1]}"

    result = f"https://github.com/kxue43/{% .ProjectName %}/blob/main/{filename}.py{anchor}"
    return result
