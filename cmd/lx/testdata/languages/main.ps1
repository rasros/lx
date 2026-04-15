<#
  Module: Demo utilities
  Provides greeting and arithmetic helpers.
#>

# Greet someone by name.
function Get-Greeting {
    param(
        [string]$Name,
        [string]$Greeting = "Hello" # default greeting
    )
    "$Greeting, $Name"
}

function Add-Numbers {
    param([int]$A, [int]$B)
    $A + $B
}

function helper { # internal
    "internal"
}
