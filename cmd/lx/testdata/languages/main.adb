with Ada.Text_IO; use Ada.Text_IO;

package Demo is
   procedure Greet(Name : String);
   function Add(A, B : Integer) return Integer;
   function Combine(
      A : String;
      B : String) return String;
end Demo;

procedure Greet(Name : String) is
begin
   Put_Line("Hello, " & Name);
end Greet;

function Add(A, B : Integer) return Integer is
begin
   return A + B;
end Add;

function Combine(
   A : String;
   B : String) return String is
begin
   return A & ", " & B;
end Combine;
