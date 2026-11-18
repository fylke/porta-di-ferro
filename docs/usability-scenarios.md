# Usability scenarios

Tasks for manual usability testing (#111). Each one is something that happens at a real event,
given to a tester in the words an organizer would hear it, with nothing about where in the
application to look. Run them on a laptop with the server, a tablet as the score keeper and,
where it says so, a second device on the same wifi.

For each scenario: read the task aloud, and then say nothing. Note where the tester looks first,
where they hesitate, what they say out loud, and whether they get there. Time it. A task that
needs a hint is a finding, not a failure of the tester.

## Before the session

Start from an event under way, so the tester meets the application the way an organizer does
at ten in the morning:

- Two disciplines, Open Steel Longsword and Sword and Buckler, each with entrants and pools
  drawn, in one block of the day, on two mats.
- Pools under way on both mats, with a few matches fenced, so the mat board, the staff panel
  and the screens all have something in them.
- A staff of at least six, with Hoakim Jammarslätt as head referee on one of the Sword and
  Buckler pools, and the score keeper tablet signed in to mat 1.
- A third mat in the hall with nothing on it.

The public demo (docs/demo.md) is a quick way to get most of this; the steps below name the
pages a tester would find, for whoever writes up the session.

## 1. A short extra tournament on the empty mat

> "In the middle of the event, Liro Machtela and four friends ask if they can run a short Sabre
> and Buckler tournament on the empty mat. Set it up for them."

What it takes:

- Add a discipline (Plan the event → Disciplines, or the admin page), Sabre and Buckler.
- Enter five competitors: Liro Machtela and four more.
- Set it up for one mat with pools of five, and draw the pools — one pool of five, ten matches.
- Put its pool on the empty mat (the mat board on the admin page, On the day: drag the card, or
  its menu's "To the end of mat 3"). A mat does not start another discipline's work by itself,
  so the mat board shows the mat stopped in front of it: press **Start**.
- Point a score keeper at mat 3.

Success: the score keeper on mat 3 shows Liro's first match without the tester touching the
other disciplines' plan, and the two running disciplines carry on undisturbed.

Watch for: whether the tester expects a separate "quick tournament" button; whether they find
the mat board; whether the go / no-go warnings on the new discipline's page (no staff, a mat
short) read as blocking — they are not.

## 2. A head referee is out

> "Hoakim Jammarslätt was going to be head referee in one of the Sword and Buckler pools, but
> he's just had a concussion. Make sure there's still enough staff to run the tournament."

What it takes:

- Find Hoakim on the staff panel (the admin page, Before the day → Staff) and take him off:
  Remove, or take his slots away by hand.
- See what is now short — the panel's list, and "Short of head referee" on the pool's card on
  the mat board — and Sword and Buckler's go / no-go summary.
- Fill the slot: **Suggest who works where**, look at what it proposes, **Apply**; or put
  somebody in the slot by hand.

Success: every slot of the Sword and Buckler pools is filled again, by somebody not fencing at
the time, and the tester can say who replaced Hoakim.

Watch for: whether the tester looks for "unavailable" rather than "remove"; whether they trust
the suggestion or check it; whether they notice the suggestion never puts a competitor on a
mat while they fence.

## 3. Correcting an exchange five exchanges back

> "Score keeper Wontus Pesterholm has just noticed he made a mistake in the very first exchange
> of the match. It's the fifth exchange now. He asks you to change the first one from 1–1 to
> 2–1."

What it takes:

- Find the match: the discipline's admin page, its pool, the match row's edit button (the
  pencil), which opens the match's log.
- Change the first exchange's red value from 1 to 2, and save. The previous log is kept as a
  backup beside it.
- On the score keeper's tablet, the match reloads with the corrected score and carries on from
  the fifth exchange.

Success: the score reads one point higher for red on the tablet, on the mat's screen and in the
pool table, the match goes on, and the tester can say the old version was not lost.

Watch for: whether the tester tries to do it on the tablet (Undo only takes back the last
exchange — a finding if they expect more); whether the log editor's columns make sense
without explanation; whether they worry about the score keeper's tablet disagreeing.

## After the session

File what was found as issues, one per finding, with the scenario's number and what the tester
said at the moment it happened. A change to the application that changes how a scenario is done
changes this page too.
