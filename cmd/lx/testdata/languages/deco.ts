@Component({ selector: "app" })
export class Widget {
    @Input() title: string = "";

    @HostListener("click")
    onClick(): void {}
}
