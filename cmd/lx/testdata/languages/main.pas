program Demo;

{ Animal record type. }
type
  TAnimal = record
    Name: string;    (* display name *)
    Species: string; (* taxonomy     *)
  end;

// Greet by name.
procedure Greet(Name: string);
begin
  WriteLn('Hello, ', Name);
end;

(* Add two integers. *)
function Add(A, B: Integer): Integer;
begin
  Result := A + B;
end;

{ Combine two strings with a separator. }
function Combine(
  A: string;
  B: string): string;
begin
  Result := A + ', ' + B;
end;

begin
end.
