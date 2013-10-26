var hakobiyaModule = angular.module('hakobiya', []);

hakobiyaModule.factory('Hakobiya', function($rootScope) {
	var Hakobiya = {
		socket: null,
		sendQueue: [],
		channels: [],
		jpCount: {},
		URL: null,

		connect: function(addr) {
			this.URL = addr;
			this.socket = new WebSocket(addr);

			var self = this;
			this.socket.onopen = function() {
				console.log("Hakobiya: connected.");

				angular.forEach(self.sendQueue, function(msg) {
				    self.send(msg);
				});
				self.sendQueue = [];
			}
			this.socket.onclose = function() {
				console.log("Hakobiya: disconnected.");
			}
			this.socket.onerror = function (error) {
			    console.log(error);
			}
			this.socket.onmessage = function(evt) {
				var data = angular.fromJson(evt.data);
				switch (data.x) {
					case 's': //set
						var qualified = data.c + "." + data.n;
						$rootScope.$broadcast(qualified, data.v);
						break;
					case '!': //error
						console.log(data);
						break;
				}
			}
		},
		send: function(data) {
			if (this.socket && this.socket.readyState == 1) {
				this.socket.send(angular.toJson(data));
			} else {
				this.sendQueue.push(data);
			}
		},
		join: function(channel) {
			if (!this.jpCount[channel] || this.jpCount[channel] == 0) {
				this.send({
					x: 'j',
					c: channel
				});
				this.jpCount[channel] = 1;
			} else {
				this.jpCount[channel] += 1;
			}
		},
		part: function(channel) {
			this.jpCount[channel] -= 1;
			if (this.jpCount[channel] < 1) {
				this.send({
					x: 'p',
					c: channel
				});
			}
		},
		get: function(channel, vars) {
			this.send({
				x: 'g',
				c: channel,
				n: vars
			});
		},
		set: function(channel, variable, value) {
			this.send({
				x: 's',
				c: channel,
				n: variable,
				v: value
			});
		},
		bind: function($scope, chvar, binding) {
			// is our channel null? if so we have to do this later...
			var ch = $scope[chvar];
			if (!ch) {
				var self = this;
				var unreg = $scope.$watch(chvar, function(newVal, oldVal) {
					self._do_bind($scope, newVal, binding);
					// for now, we only do this once
					// TODO: changing channels
					unreg();
				});
			} else {
				this._do_bind($scope, ch, binding);
			}
		},
		// ----- here be dragons -----
		_do_bind: function($scope, chan, binding) {
			var self = this;

			this.join(chan);
			$scope.$on("$destroy", function() {
				self.part(chan);
			});

			// here we go...
			var hvars = [];
			angular.forEach(binding, function(hvar, scopevar) {
				hvars.push(hvar);
				var sigil = hvar[0];
				var qualified = chan + "." + hvar;
				switch (sigil) {
					case '&': // magic var, one-way server -> client binding
					case '$': // system var, same
						$scope.$on(qualified, function(e, value) {
							$scope.$apply(function(scope) {
								scope[scopevar] = value;
							});
						});
						break;
					case '%': // uservars, two-way binding
						$scope.$on(qualified, function(e, value) {
							$scope.$apply(function(scope) {
								scope[scopevar] = value;
							});
						});
						$scope.$watch(scopevar, function(newVal, oldVal) {
							self.set(chan, hvar, newVal);
						});
						break;
					case '#': // broadcasts, one way for now (TODO)
						$scope.$on(qualified, function(e, value) {
							$scope.$apply(function(scope) {
								if (scope[scopevar]) {
									scope[scopevar].push(value);
								} else {
									scope[scopevar] = [value];
								}
							});
						});
						break;
					default:
						console.log("ERROR: unknown sigil " + hvar);
						break;
				}
			});
			this.get(chan, hvars);	
		}
	};
	return Hakobiya;
});