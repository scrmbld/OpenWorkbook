const path = require("path")

module.exports = {
	entry: {
		index: './src/js/index.js',
		termHeader: './src/js/termHeader.js',
		termBody: './src/js/termBody.js',
	},
	output: {
		filename: '[name].bundle.js',
		path: path.resolve(__dirname, '../static'),
	},
	module: {
		rules: [
			{
				test: /\.css/i,
				use: ['style-loader', 'css-loader'],
			},
		],
	},
};
