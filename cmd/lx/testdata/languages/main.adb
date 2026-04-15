with Ada.Text_IO; use Ada.Text_IO;

-- Demo package specification.
package Demo is
   procedure Greet(Name : String);
   function Add(A, B : Integer) return Integer;
   -- Multi-line signature.
   function Combine(
      A : String;
      B : String) return String;
end Demo;

-- Greet by printing to output.
procedure Greet(Name : String) is
begin
   Put_Line("Hello, " & Name);
end Greet;

-- Add two integers.
function Add(A, B : Integer) return Integer is
begin
   return A + B;
end Add;

-- Combine two strings with a separator.
function Combine(
   A : String;
   B : String) return String is
begin
   return A & ", " & B;
end Combine;
