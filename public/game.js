class Game extends HTMLElement {
	constructor() {
		super();
		this.attachShadow({ mode: 'open' });
	}

	async connectedCallback() {
		let data = await fetch('/room/list').then(async r => {
			return await r.json().then(data => {return data});
		})

		data.forEach((room) => {
			this.shadowRoot.append(`
			<div class="room">
				<p>${room.name}</p>
				<button>Connect</button>
			</div>
			`)
		})

		/* Canvas
		this.shadowRoot.innerHTML = `
          <canvas id="game-window"></canvas>
        `;
		 */
	}

	disconnectedCallback() {
		this.shadowRoot.innerHTML = '';
		console.log('Game destroyed');
	}
}

customElements.define('game-window', Game);