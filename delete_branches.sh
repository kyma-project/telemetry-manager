#!/bin/bash
    # Get a list of local branches merged into main

   branches=$(git branch | grep -v 'main$' | grep -v 'release-*' | xargs)

    # Delete merged branches
    if [ -n "$branches" ]; then
        git branch -D $branches
    else
        echo "No merged branches to delete."
    fi
