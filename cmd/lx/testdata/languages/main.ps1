function Get-Greeting {
    param(
        [string]$Name,
        [string]$Greeting = "Hello"
    )
    "$Greeting, $Name"
}

function Add-Numbers {
    param([int]$A, [int]$B)
    $A + $B
}

function helper {
    "internal"
}
