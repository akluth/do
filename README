## do - do ist, just, just...do it.

### What is this?

This is nothing more, nothing less than a simple and stupid command-processor,
task executer, /just-execute-shell-commands-in-order-grouped-by-tasks-thingy or
whatever you like to call it.

No logo, no code coverage, no 'backers', just some Go code hacked together in around
an hour to avoid using Makefiles and make anymore and all of the alternatives out there.

It does the job for me and maybe it does it for you.


### Usage

Dofile:

    description = "Very important tasks"

    [tasks]

        [tasks.build]
        commands = [
            "my-super-cool-build-tool --with-options AndArguments"
        ]

        [tasks.clean]
        commands = [
            "rm -rf /",
            "echo 'That wasn't so smart, wasn't it?'"
        ]

Then simply type

    $ do build
    $ do clean
    $ do build clean

to execute those super-important tasks.


