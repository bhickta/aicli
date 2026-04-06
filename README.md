# AI CLI

A highly extendable, SOLID, Open-Closed Python CLI built with [Typer](https://typer.tiangolo.com/) and [Rich](https://rich.readthedocs.io/).
It leverages local AI through LM Studio to perform tasks like intelligent image renaming.

## Setup

1. Install dependencies:
   ```bash
   python -m venv venv
   source venv/bin/activate
   pip install -r requirements.txt
   ```
2. Run LM Studio with a vision capable model loaded. Make sure the Local Server is running on \`http://localhost:1234/v1\`.

## Usage

Basic usage to rename an image:

```bash
python -m aicli image rename path/to/image.jpg
```
