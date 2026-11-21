# Requirements

What Porta di Ferro does, one record per requirement, each traced to the code that
implements it and a test that verifies it. How the tracing works, and how to change a
requirement, is in [`docs/requirements-as-code.md`](../docs/requirements-as-code.md).

If you are new here, read the files in this order:

1. [`porta.rsl`](porta.rsl): the kinds of requirement, and what every record carries.
2. [`stories.trlc`](stories.trlc): what the organizer, the score keeper, a competitor and
   a spectator do with the app.
3. [`functional.trlc`](functional.trlc): the rules. Start with the match, which is the
   heart of it.
4. [`non-functional.trlc`](non-functional.trlc): offline, secure, installable, readable.
5. [`constraints.trlc`](constraints.trlc): the rules for the shape of the code that keep
   the rest true.

To see what implements a requirement, search the repository for its ID (`FR_MATCH_03`), or
run `sh req/trace.sh` and open `build/req/report.html`.
