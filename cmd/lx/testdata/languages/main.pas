program Demo;

type
  TAnimal = record
    Name: string;
    Species: string;
  end;

procedure Greet(Name: string);
begin
  WriteLn('Hello, ', Name);
end;

function Add(A, B: Integer): Integer;
begin
  Result := A + B;
end;

function Combine(
  A: string;
  B: string): string;
begin
  Result := A + ', ' + B;
end;

begin
end.
