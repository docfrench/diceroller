[![CI](https://github.com/docfrench/diceroller/actions/workflows/ci.yml/badge.svg)](https://github.com/docfrench/diceroller/actions/workflows/ci.yml) [![Go Reference](https://go.dev)](https://go.dev) ![GitHub Go version](https://shields.io)

# diceroller

A single page app written in Golang.

Rolls dice for the Storyteller ttRPG system.

## Use

Complete the following fields and click Roll

- Character Name (optional)
- Reason (optional)
- Dice, in format 5d10
- Difficulty
- Table passphrase

The app will roll the requested dice, sort them, and tell the user how many successes they earned. The roll events are recorded in a persistent sqlite database for future review.

