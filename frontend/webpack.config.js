const path = require("path")

module.exports = {
	mode: 'development',
	entry: {
		index: './src/index.js',
		term: './src/term.js',
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
