<template>
	<div class="grid">
		<div class="col-12">
			<div class="card">
				<h5>About</h5>

				<Accordion>
					<AccordionTab header="Build Info">
						<pre><code v-text="aboutBuild"></code></pre>
					</AccordionTab>
					<AccordionTab header="System Info">
						<pre><code v-text="aboutSystem"></code></pre>
					</AccordionTab>
				</Accordion>
			</div>
		</div>
	</div>
</template>

<script lang="ts">
import { defineComponent, ref } from 'vue'
import API from '../../api/api'

export default defineComponent({
	data() {
		return {
			api: new API(),
			aboutBuild: ref('no build info'),
			aboutSystem: ref('no system info'),
			timer: 0
		}
	},
	mounted() {
		this.startAutoUpdate()
	},
	beforeUnmount () {
		this.cancelAutoUpdate()
	},
	methods: {
		updatePageData () {
			this.api.fetchAbout().then((data) => {
				this.aboutBuild = JSON.stringify(data['build'], undefined, 4)
				this.aboutSystem = JSON.stringify(data['system'], undefined, 4)
			});
		},
		startAutoUpdate() {
			this.updatePageData()
			this.timer = setInterval(this.updatePageData, 5000)
		},
		cancelAutoUpdate() {
			clearInterval(this.timer)
		}
	},
})
</script>