<template>
	<div class="layout-footer">
		<img alt="Logo" :src="footerImage()" height="20" class="mr-2" />
		by
		<a href="https://dev.l1qu1d.net/wraith-labs/"><span class="font-medium ml-2">Wraith Labs</span></a>
	</div>
</template>

<script>
	export default {
		name: "AppFooter",
		methods: {
			footerImage() {
				return 'images/logo.png';
			}
		},
		computed: {
			darkTheme() {
				return this.$appState.darkTheme;
			}
		}
	}
</script>