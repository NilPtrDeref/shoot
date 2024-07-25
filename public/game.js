class Game extends HTMLElement {
  constructor() {
    super();
    this.movement = { up: false, down: false, left: false, right: false };

    this.handle_keypress = this.handle_keypress.bind(this);
    this.handle_message = this.handle_message.bind(this);
  }

  handle_keypress(event) {
    switch (event.key) {
      case "w" || "ArrowUp":
        this.movement.up = event.type === "keydown";
        break;
      case "s" || "ArrowDown":
        this.movement.down = event.type === "keydown";
        break;
      case "a" || "ArrowLeft":
        this.movement.left = event.type === "keydown";
        break;
      case "d" || "ArrowRight":
        this.movement.right = event.type === "keydown";
        break;
    }
  }

  handle_message(message) {
    let data = JSON.parse(message.data);

    if (data.type === "join") {
      console.log(data);
    }

    if (data.type === "update") {
      console.log(data);
    }

    // TODO: Handle errors appropriately
    if (data.type === "error") {
      console.log(data);
      this.ws.close();
    }
  }

  connectedCallback() {
    this.attachShadow({ mode: "open" });

    this.shadowRoot.innerHTML = `
      <link rel="stylesheet" href="/public/global.css">
      <canvas id="game-window"></canvas>
    `;

    this.canvas = this.shadowRoot.querySelector("#game-window");
    this.ctx = this.canvas.getContext("2d");
    this.room = this.getAttribute("room");
    this.ws = new WebSocket(`/room/${this.room}/ws`);

    this.ws.addEventListener("message", this.handle_message);

    // TODO: On disconnect, try reconnect?
    // this.ws.addEventListener("close", () => {
    //   location.reload();
    // });

    window.addEventListener("keydown", this.handle_keypress);
    window.addEventListener("keyup", this.handle_keypress);
  }

  disconnectedCallback() {
    window.removeEventListener("keydown", this.handle_keypress);
    window.removeEventListener("keyup", this.handle_keypress);

    this.ws.close();
    this.shadowRoot.innerHTML = "";
  }
}

customElements.define("game-window", Game);
