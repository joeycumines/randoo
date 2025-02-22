# randoo

randoo is a simple command-line utility written in Go that randomizes the order of command arguments before executing a
given command. It provides flexible options to control which arguments get shuffled, whether by shuffling all arguments
or only a subset delimited by special tokens.

---

## Overview

By design, randoo:

- **Shuffles arguments:** It randomizes the order of command arguments.
- **Executes commands:** After shuffling, it calls the specified command with the randomized arguments.
- **Offers selective shuffling:** You can control which arguments are shuffled using start (`-s`) and end (`-e`)
  delimiters (consumed).
- **Supports input from stdin:** With the `-l` flag, you can supply additional arguments via standard input (one per
  line).

---

## Usage

```bash
randoo [options] [--] command [args...]
```

- **command:** The executable you want to run.
- **args...:** The arguments for the command, which will be randomized according to the options provided.

---

## Command-Line Options

- **`-s string`**
  *Shuffle args after the specified argument (start delimiter).*
  The argument provided with this flag marks the beginning of the segment to be shuffled.
  **Behavior:**
    - The tool searches for this delimiter in the provided arguments.
    - If found, the arguments after this token (or up to an end delimiter, if specified) are shuffled.
    - The delimiter itself is not passed on to the command.
    - **Error:** If the token is not found, randoo exits with an error.

- **`-e string`**
  *Shuffle args before the specified argument (end delimiter).*
  This flag designates the end of the segment to be shuffled.
  **Behavior:**
    - The tool searches for this token.
    - If found, the arguments preceding it (or a segment defined between a start and this end delimiter) are randomized.
    - Like the start delimiter, this token is not passed on.
    - **Error:** If the token is not found, an error is reported.

- **`-l`**
  *Read input from stdin, one argument per line.*
  **Behavior:**
    - Reads additional arguments from standard input.
    - These arguments are appended to any trailing arguments (which are not shuffled unless affected by the delimiters).
    - This is useful for dynamically supplying arguments at runtime.

---

## Implementation Details

### Shuffling Logic

- **Complete Shuffle:**
  If neither `-s` nor `-e` is provided, all arguments are shuffled.

- **Single Delimiter Mode:**
  When only one of the delimiters is specified:
    - For **`-s` only:** randoo finds the delimiter, removes it, and shuffles all arguments that follow.
    - For **`-e` only:** It shuffles all arguments preceding the found delimiter and then removes the delimiter.

- **Delimited Range Shuffle:**
  If **both** `-s` and `-e` are provided:
    - The program searches for the start delimiter (`-s`).
    - It then looks for the end delimiter (`-e`) after the start delimiter.
    - Only the arguments in between are shuffled.
    - After shuffling, both delimiters are removed before execution.

- **Random Source:**
  randoo uses a custom random source based on `crypto/rand` to seed the shuffle mechanism, ensuring that the
  randomization is well-seeded and secure.

### Execution and Signal Handling

- **Command Execution:**
  After processing and shuffling the arguments, randoo executes a subprocess, with the provided command, and processed
  arguments.

- **Signal Forwarding:**
  It sets up signal forwarding so that any signals received (like SIGINT) are passed to the spawned process, ensuring
  proper signal handling during execution.

- **Error Handling:**
  The tool validates that:
    - A command is provided.
    - Required delimiters exist in the arguments (if specified).

  In case of errors (e.g., missing command, missing delimiter), it outputs a relevant error message and exits with a
  non-zero status. It also attempts to propagate the exit code of the executed command, when possible.

---

## Examples

### 1. Basic Randomization

Shuffle all arguments before executing the command:

```bash
randoo cat file1.txt file2.txt file3.txt
```

This will shuffle the order of `file1.txt`, `file2.txt`, and `file3.txt` before concatenating them.

### 2. Using a Single Delimiter

Shuffle only the arguments after a specified token:

```bash
randoo -s START echo a b c START arg1 arg2 arg3
```

Example output: `a b c arg2 arg3 arg1`

- **Behavior:**
    - `START` is used as the start delimiter.
    - The arguments following `START` (`arg1 arg2 arg3`) are shuffled.
    - The delimiter is removed before executing the command.

### 3. Using Both Delimiters

Shuffle a specific range of arguments:

```bash
randoo -s START -e END echo a b c START arg1 arg2 arg3 END d e
```

Example output: `a b c arg2 arg3 arg1 d e`

- **Behavior:**
    - `START` marks the beginning and `END` marks the end of the segment to be shuffled.
    - Only the arguments between `START` and `END` (`arg1 arg2 arg3`) are randomized.
    - Both delimiters are stripped from the final command line.

### 4. Reading Arguments from Stdin

Shuffle arguments provided via standard input:

```bash
echo -e "arg1\narg2\narg3" | randoo -l mycommand
```

- **Behavior:**
    - Reads each line from stdin as an argument.
    - Shuffles the input arguments.
    - Appends them to the command `mycommand` for execution.
