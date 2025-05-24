# greplace

A port of [replace.c](https://github.com/twitter-forks/mysql/blob/master/extra/replace.c).

> The replace utility program changes strings in place in files or on the standard input.

```
shell> replace from to [from to] ... -- file_name [file_name] ...
shell> replace from to [from to] ... < file_name
```

from represents a string to look for and to represents its replacement. There
can be one or more pairs of strings [...] The replace program is used by
msql2mysql. See msql2mysql(1).


Background: [Optimize shell script for multiple sed replacements](https://stackoverflow.com/a/25563135/89391), asked Aug 29, 2014 at 6:50.


## Background

Trying to use LLM for porting C to Go. Claude 4 (2025-05-23) got to a compilable state, but:

* had memory issues
* had bugs
* overlooked a simple precondition of the input data (data with newlines, reasonable line length)

Retry with Gemini.

> Porting this code will be a significant effort, especially the DFA
> construction and string replacement logic, but it's a good learning exercise
> for understanding Go's idioms and memory model compared to C. **Good luck!**

## Notes

### Canvas

In canvas mode, gemini will compile the code and will feed error messages back,
seemingly until the compilation succeeds. It is like a small in chat agent.
