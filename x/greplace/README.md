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

## Benchmarks

| sed        | 1 | 1G | 3.733s  |
|------------|---|----|---------|
| greplace   | 1 | 1G | 8.158s  |
| replace(1) | 1 | 1G | 7.815s  |
| sed        | 5 | 1G | 11.016s |
| greplace   | 5 | 1G | 17.740s |
| replace(1) | 5 | 1G | 7.976s  |

Single string in a 1G file:

```shell
$ hyperfine "sed 's@Kramer@KRAMER@' m.txt > /dev/null"
Benchmark 1: sed 's@Kramer@KRAMER@' m.txt > /dev/null
  Time (mean ± σ):      3.733 s ±  0.113 s    [User: 3.493 s, System: 0.239 s]
  Range (min … max):    3.589 s …  3.933 s    10 runs

$ hyperfine "./greplace Kramer KRAMER < m.txt > /dev/null"
Benchmark 1: ./greplace Kramer KRAMER < m.txt > /dev/null
  Time (mean ± σ):      8.158 s ±  0.348 s    [User: 7.983 s, System: 1.447 s]
  Range (min … max):    7.639 s …  8.752 s    10 runs

$ hyperfine "replace Kramer KRAMER < m.txt > /dev/null"
Benchmark 1: replace Kramer KRAMER < m.txt > /dev/null
  Time (mean ± σ):      7.815 s ±  0.251 s    [User: 6.046 s, System: 1.769 s]
  Range (min … max):    7.310 s …  8.033 s    10 runs
```

Replace five strings at once:

```shell
$ hyperfine "sed 's@Kramer@KRAMER@;s@space@SPACE@;s@human@HUMAN@;s@vidphone@VIDPHONE@;s@computer@COMPUTER@' m.txt > /dev/null "
Benchmark 1: sed 's@Kramer@KRAMER@;s@space@SPACE@;s@human@HUMAN@;s@vidphone@VIDPHONE@;s@computer@COMPUTER@' m.txt > /dev/null
  Time (mean ± σ):     11.016 s ±  0.182 s    [User: 10.763 s, System: 0.252 s]
  Range (min … max):   10.710 s … 11.365 s    10 runs

$ hyperfine "./greplace Kramer KRAMER space SPACE human HUMAN vidphone VIDPHONE computer COMPUTER < m.txt > /dev/null "
Benchmark 1: ./greplace Kramer KRAMER space SPACE human HUMAN vidphone VIDPHONE computer COMPUTER < m.txt > /dev/null
  Time (mean ± σ):     17.740 s ±  0.447 s    [User: 17.698 s, System: 1.371 s]
  Range (min … max):   17.167 s … 18.761 s    10 runs

$ hyperfine "replace Kramer KRAMER space SPACE human HUMAN vidphone VIDPHONE computer COMPUTER < m.txt > /dev/null "
Benchmark 1: replace Kramer KRAMER space SPACE human HUMAN vidphone VIDPHONE computer COMPUTER < m.txt > /dev/null
  Time (mean ± σ):      7.976 s ±  0.140 s    [User: 6.157 s, System: 1.818 s]
  Range (min … max):    7.710 s …  8.143 s    10 runs
```


## Notes

### Memory leaks

The first working go version would be limited by memory (never releases data, even in stream mode).

### Project token size

```
$ make digest.txt
uvx gitingest -o digest.txt -e testdata
Analysis complete! Output written to: digest.txt

Summary:
Directory: greplace
Files analyzed: 13

Estimated tokens: 22.7k
```

### Who is the I?

> Should I start with the streaming buffer management, or would you prefer to
> first fix the DFA execution to use the proper state machine instead of brute
> force?

### Too buggy, too overengineered

Claude 4 on code it wrote:

> The problem might be that your DFA construction is too complex and buggy.
> Let's create a minimal working version ...

#### Key Insights

> Your streaming is over-engineered: The C code only does line-based streaming,
> and for inputs without newlines, it reads everything into memory anyway.

### You're right to question this

> You're absolutely right to question this!

So hard to trust.

> Excellent observation! I **completely misunderstood the C algorithm**
> initially.  The C code actually uses a line-by-line streaming approach with
> fixed memory usage.

Oder: wer einmal lügt, dem glaubt man nicht ([DWDS](https://www.dwds.de/wb/wer%20einmal%20l%C3%BCgt%2C%20dem%20glaubt%20man%20nicht))


### Learning

> **Now I understand completely!** The C code's memory efficiency depends on
> the input having reasonable line lengths. For your test case (which has no
> newlines), the C code actually does read the entire input into memory.


### A view of software

In which domain is a piece of code considered to be done after it initially
works and has no tests? Maybe in the same place, where many models originate
from.
