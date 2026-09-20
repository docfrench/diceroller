[![CI](https://github.com/docfrench/diceroller/actions/workflows/ci.yml/badge.svg)](https://github.com/docfrench/diceroller/actions/workflows/ci.yml) 

![GitHub Actions](https://img.shields.io/badge/github%20actions-%232671E5.svg?style=for-the-badge&logo=githubactions&logoColor=white)

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/docfrench/diceroller)



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

